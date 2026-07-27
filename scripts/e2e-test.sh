#!/usr/bin/env bash
#
# linko end-to-end test
#
# Builds linko, runs the unit tests, then drives a full real-world cycle
# against your Cloudflare account:
#
#   init -> start -> fetch the public URL -> list/status/doctor -> remove
#
# It uses a throwaway config directory (LINKO_HOME), so your real
# ~/.linko/config.json is never touched.
#
# Usage:
#   export LINKO_API_TOKEN=...
#   ./scripts/e2e-test.sh
#
# Configuration (all optional):
#   LINKO_API_TOKEN   Cloudflare API token                 (required)
#   E2E_DOMAIN        zone apex          default techkahwa.net
#   E2E_BASE          base subdomain     default linko.techkahwa.net
#   E2E_PORT          local port to expose               default 3000
#   E2E_NAME          subdomain label                 default memento
#   E2E_KEEP          set to 1 to skip the cleanup step

set -uo pipefail

DOMAIN="${E2E_DOMAIN:-techkahwa.net}"
BASE="${E2E_BASE:-linko.techkahwa.net}"
PORT="${E2E_PORT:-3000}"
NAME="${E2E_NAME:-memento}"
HOSTNAME_FQDN="${NAME}.${BASE}"
PUBLIC_URL="https://${HOSTNAME_FQDN}"
LOCAL_URL="http://localhost:${PORT}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${ROOT}/linko"
WORK="$(mktemp -d)"
export LINKO_HOME="${WORK}/home"
TUNNEL_LOG="${WORK}/tunnel.log"
TUNNEL_PID=""

PASS=0
FAIL=0

bold()  { printf '\033[1m%s\033[0m\n'  "$*"; }
dim()   { printf '\033[2m%s\033[0m\n'  "$*"; }
ok()    { printf '\033[32m✓\033[0m %s\n' "$*"; PASS=$((PASS+1)); }
bad()   { printf '\033[31m✗\033[0m %s\n' "$*"; FAIL=$((FAIL+1)); }
warn()  { printf '\033[33m!\033[0m %s\n' "$*"; }
step()  { printf '\n\033[1m── %s\033[0m\n' "$*"; }

cleanup() {
  if [ -n "$TUNNEL_PID" ] && kill -0 "$TUNNEL_PID" 2>/dev/null; then
    dim "stopping the tunnel (pid $TUNNEL_PID)"
    kill -INT "$TUNNEL_PID" 2>/dev/null
    for _ in $(seq 1 20); do
      kill -0 "$TUNNEL_PID" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "$TUNNEL_PID" 2>/dev/null
  fi
  dim "work dir: $WORK"
}
trap cleanup EXIT

# ---------------------------------------------------------------- preflight

step "0 · Preflight"

command -v go   >/dev/null 2>&1 || { bad "go is not installed"; exit 1; }
command -v curl >/dev/null 2>&1 || { bad "curl is not installed"; exit 1; }
ok "go $(go version | awk '{print $3}')"

if [ -z "${LINKO_API_TOKEN:-}" ]; then
  bad "LINKO_API_TOKEN is not set"
  echo "    export LINKO_API_TOKEN='your-cloudflare-token'"
  exit 1
fi
ok "LINKO_API_TOKEN is set (${#LINKO_API_TOKEN} characters)"

if curl -fsS --max-time 4 -o /dev/null "$LOCAL_URL"; then
  ok "your app is responding on ${LOCAL_URL}"
else
  bad "nothing is responding on ${LOCAL_URL} — start your project first"
  exit 1
fi

LOCAL_BODY="$(curl -fsS --max-time 8 "$LOCAL_URL" | head -c 2000)"

# ------------------------------------------------------------ build + units

step "1 · Build and unit tests"

( cd "$ROOT" && go mod tidy ) >"${WORK}/tidy.log" 2>&1 \
  && ok "go mod tidy" \
  || { bad "go mod tidy failed"; tail -20 "${WORK}/tidy.log"; exit 1; }

( cd "$ROOT" && gofmt -s -l . ) >"${WORK}/fmt.log" 2>&1
if [ -s "${WORK}/fmt.log" ]; then
  warn "these files are not gofmt'd:"; cat "${WORK}/fmt.log"
else
  ok "gofmt clean"
fi

( cd "$ROOT" && go vet ./... ) >"${WORK}/vet.log" 2>&1 \
  && ok "go vet" \
  || { bad "go vet failed"; cat "${WORK}/vet.log"; exit 1; }

( cd "$ROOT" && go test ./... -count=1 ) >"${WORK}/test.log" 2>&1 \
  && ok "unit tests ($(grep -c '^ok' "${WORK}/test.log") packages)" \
  || { bad "unit tests failed"; cat "${WORK}/test.log"; exit 1; }

( cd "$ROOT" && go build -trimpath -o "$BIN" . ) \
  && ok "built $BIN" \
  || { bad "build failed"; exit 1; }

"$BIN" --version >/dev/null 2>&1 && ok "linko --version runs" || bad "linko --version failed"

# ------------------------------------------------------------------- init

step "2 · linko init"

if "$BIN" init --yes \
      --domain "$DOMAIN" \
      --base "$BASE" \
      --tunnel "linko-e2e-tunnel" \
      >"${WORK}/init.log" 2>&1; then
  ok "init completed"
  sed 's/^/    /' "${WORK}/init.log"
else
  bad "init failed"
  sed 's/^/    /' "${WORK}/init.log"
  exit 1
fi

CONFIG="${LINKO_HOME}/config.json"
[ -f "$CONFIG" ] && ok "config written to \$LINKO_HOME/config.json" || bad "no config file"

if [ "$(uname -s)" != "Darwin" ]; then
  PERM="$(stat -c '%a' "$CONFIG" 2>/dev/null)"
else
  PERM="$(stat -f '%Lp' "$CONFIG" 2>/dev/null)"
fi
[ "$PERM" = "600" ] && ok "config permissions are 0600" || bad "config permissions are $PERM, expected 600"

grep -q '"tunnel_id"' "$CONFIG" && ok "tunnel id stored" || bad "no tunnel id in the config"

# ------------------------------------------------------------------- start

step "3 · linko ${PORT} --name ${NAME}"

"$BIN" "$PORT" --name "$NAME" --keep --yes >"$TUNNEL_LOG" 2>&1 &
TUNNEL_PID=$!
dim "tunnel pid $TUNNEL_PID · log $TUNNEL_LOG"

CONNECTED=0
for _ in $(seq 1 60); do
  kill -0 "$TUNNEL_PID" 2>/dev/null || break
  if grep -q "Tunnel connected" "$TUNNEL_LOG" 2>/dev/null; then CONNECTED=1; break; fi
  sleep 1
done

if [ "$CONNECTED" = "1" ]; then
  ok "cloudflared reported a connection"
else
  bad "the tunnel did not connect within 60s"
  sed 's/^/    /' "$TUNNEL_LOG"
  exit 1
fi

grep -q "$PUBLIC_URL" "$TUNNEL_LOG" \
  && ok "printed the expected URL ${PUBLIC_URL}" \
  || warn "expected ${PUBLIC_URL} in the output"

# --------------------------------------------------------------- the real test

step "4 · Fetch the public URL"

STATUS=""
for i in $(seq 1 30); do
  STATUS="$(curl -s -o "${WORK}/remote.html" -w '%{http_code}' --max-time 10 "$PUBLIC_URL" || echo 000)"
  case "$STATUS" in
    200|301|302|304) break ;;
  esac
  dim "attempt ${i}: HTTP ${STATUS} — waiting for DNS/edge propagation"
  sleep 4
done

if [ "$STATUS" = "200" ]; then
  ok "${PUBLIC_URL} returned HTTP 200"
else
  bad "${PUBLIC_URL} returned HTTP ${STATUS}"
  head -c 400 "${WORK}/remote.html" | sed 's/^/    /'
fi

REMOTE_BODY="$(head -c 2000 "${WORK}/remote.html")"
if [ "$REMOTE_BODY" = "$LOCAL_BODY" ]; then
  ok "the public response is byte-identical to ${LOCAL_URL}"
else
  warn "bodies differ (normal for apps that render per-request state)"
  dim "  local:  $(printf '%s' "$LOCAL_BODY"  | head -c 80 | tr -d '\n')"
  dim "  public: $(printf '%s' "$REMOTE_BODY" | head -c 80 | tr -d '\n')"
fi

CF_RAY="$(curl -sI --max-time 10 "$PUBLIC_URL" | tr -d '\r' | awk -F': ' 'tolower($1)=="cf-ray"{print $2}')"
[ -n "$CF_RAY" ] && ok "served through Cloudflare (cf-ray ${CF_RAY})" || warn "no cf-ray header"

SCHEME_OK="$(curl -s -o /dev/null -w '%{scheme}:%{ssl_verify_result}' --max-time 10 "$PUBLIC_URL")"
[ "$SCHEME_OK" = "https:0" ] && ok "valid TLS certificate" || warn "TLS check returned ${SCHEME_OK}"

# --------------------------------------------------------------- other commands

step "5 · list / status / doctor"

if "$BIN" list >"${WORK}/list.log" 2>&1 && grep -q "$HOSTNAME_FQDN" "${WORK}/list.log"; then
  ok "list shows ${HOSTNAME_FQDN}"
  sed 's/^/    /' "${WORK}/list.log"
else
  bad "list did not show the route"; sed 's/^/    /' "${WORK}/list.log"
fi

if "$BIN" list --remote >"${WORK}/list-remote.log" 2>&1 && grep -q "$HOSTNAME_FQDN" "${WORK}/list-remote.log"; then
  ok "list --remote agrees with Cloudflare"
else
  bad "list --remote did not show the route"; sed 's/^/    /' "${WORK}/list-remote.log"
fi

if "$BIN" status >"${WORK}/status.log" 2>&1 && grep -q "connected" "${WORK}/status.log"; then
  ok "status reports the tunnel as connected"
  sed 's/^/    /' "${WORK}/status.log"
else
  bad "status did not report a connection"; sed 's/^/    /' "${WORK}/status.log"
fi

if "$BIN" doctor >"${WORK}/doctor.log" 2>&1; then
  ok "doctor passed every check"
  sed 's/^/    /' "${WORK}/doctor.log"
else
  bad "doctor reported a problem"; sed 's/^/    /' "${WORK}/doctor.log"
fi

# ------------------------------------------------------------------- cleanup

if [ "${E2E_KEEP:-0}" = "1" ]; then
  step "6 · Cleanup skipped (E2E_KEEP=1)"
  warn "${HOSTNAME_FQDN} is still live — remove it later with:"
  echo "    LINKO_HOME=${LINKO_HOME} ${BIN} remove ${NAME} --yes"
else
  step "6 · linko remove ${NAME}"

  if "$BIN" remove "$NAME" --yes >"${WORK}/remove.log" 2>&1; then
    ok "remove completed"
    sed 's/^/    /' "${WORK}/remove.log"
  else
    bad "remove failed"; sed 's/^/    /' "${WORK}/remove.log"
  fi

  "$BIN" list 2>/dev/null | grep -q "$HOSTNAME_FQDN" \
    && bad "the route is still listed after remove" \
    || ok "the route is gone from the local config"

  "$BIN" list --remote 2>/dev/null | grep -q "$HOSTNAME_FQDN" \
    && bad "the ingress rule still exists on Cloudflare" \
    || ok "the ingress rule is gone from Cloudflare"

  sleep 5
  AFTER="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$PUBLIC_URL" || echo 000)"
  if [ "$AFTER" = "200" ]; then
    warn "${PUBLIC_URL} still answers 200 (edge cache) — recheck in a minute"
  else
    ok "${PUBLIC_URL} no longer serves the app (HTTP ${AFTER})"
  fi
fi

# -------------------------------------------------------------------- report

step "Result"
echo "  passed: ${PASS}"
echo "  failed: ${FAIL}"
echo
if [ "$FAIL" -eq 0 ]; then
  bold "linko works end to end."
  exit 0
fi
bold "${FAIL} check(s) failed — logs are in ${WORK}"
exit 1
