#!/usr/bin/env bash
#
# linko installer
#
#   curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh | bash
#
# Environment variables:
#   LINKO_VERSION   version to install (default: latest)
#   LINKO_INSTALL   install directory  (default: /usr/local/bin, or ~/.local/bin)

set -euo pipefail

REPO="${LINKO_REPO:-Devehab/linko}"
VERSION="${LINKO_VERSION:-latest}"
BINARY="linko"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
dim()   { printf '\033[2m%s\033[0m\n' "$*"; }

die() { red "error: $*" >&2; exit 1; }

# TMP must be global: the EXIT trap fires after main() returns, so a variable
# scoped to main() would be unset by then and `set -u` would abort.
TMP=""
cleanup() { [ -n "${TMP:-}" ] && rm -rf "$TMP"; return 0; }
trap cleanup EXIT

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "$os" in
    linux|darwin) ;;
    *) die "unsupported operating system: $os (Windows users: download the .zip from https://github.com/$REPO/releases)" ;;
  esac

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) die "unsupported architecture: $arch" ;;
  esac

  echo "${os}_${arch}"
}

on_path() {
  case ":${PATH}:" in
    *":$1:"*) return 0 ;;
    *)        return 1 ;;
  esac
}

# Prefer a directory that is ALREADY on the user's PATH, so `linko` works the
# moment this script finishes. Only fall back to ~/.local/bin — which is often
# not on PATH — when nothing better is writable, and patch the shell profile
# in that case.
choose_install_dir() {
  local d
  if [ -n "${LINKO_INSTALL:-}" ]; then
    echo "$LINKO_INSTALL"; return
  fi
  for d in /opt/homebrew/bin /usr/local/bin "$HOME/.local/bin" "$HOME/bin"; do
    if on_path "$d" && [ -d "$d" ] && [ -w "$d" ]; then echo "$d"; return; fi
  done
  for d in /opt/homebrew/bin /usr/local/bin; do
    if [ -d "$d" ] && [ -w "$d" ]; then echo "$d"; return; fi
  done
  echo "$HOME/.local/bin"
}

# Append the PATH line to whichever shell profiles exist, once.
add_to_profiles() {
  local dir="$1" line rc touched=0
  line="export PATH=\"${dir}:\$PATH\""

  for rc in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
    [ -f "$rc" ] || continue
    touched=1
    grep -qF "$dir" "$rc" 2>/dev/null && continue
    printf '\n# added by the linko installer\n%s\n' "$line" >> "$rc"
    dim "added ${dir} to $(basename "$rc")"
  done

  if [ "$touched" = "0" ]; then
    rc="$HOME/.profile"
    case "${SHELL:-}" in
      */zsh)  rc="$HOME/.zshrc" ;;
      */bash) rc="$HOME/.bashrc" ;;
    esac
    printf '# added by the linko installer\n%s\n' "$line" >> "$rc"
    dim "created $(basename "$rc") with ${dir} on PATH"
  fi
}

resolve_version() {
  if [ "$VERSION" != "latest" ]; then
    echo "$VERSION"
    return
  fi
  local url tag
  url="https://api.github.com/repos/${REPO}/releases/latest"
  tag="$(curl -fsSL "$url" | grep -m1 '"tag_name"' | cut -d'"' -f4 || true)"
  [ -n "$tag" ] || die "could not determine the latest version — set LINKO_VERSION"
  echo "$tag"
}

main() {
  command -v curl >/dev/null 2>&1 || die "curl is required"
  command -v tar  >/dev/null 2>&1 || die "tar is required"

  local platform version tag archive url dest
  platform="$(detect_platform)"
  tag="$(resolve_version)"
  version="${tag#v}"

  archive="${BINARY}_${version}_${platform}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${tag}/${archive}"

  dim "Installing ${BINARY} ${tag} (${platform})"

  TMP="$(mktemp -d)"

  curl -fsSL "$url" -o "$TMP/$archive" \
    || die "download failed: $url"
  tar -xzf "$TMP/$archive" -C "$TMP"
  [ -f "$TMP/$BINARY" ] || die "the archive does not contain a ${BINARY} binary"
  chmod +x "$TMP/$BINARY"

  dest="$(choose_install_dir)"
  mkdir -p "$dest"

  if [ -w "$dest" ]; then
    mv "$TMP/$BINARY" "$dest/$BINARY"
  else
    dim "elevating with sudo to write to $dest"
    sudo mv "$TMP/$BINARY" "$dest/$BINARY"
  fi

  green "✓ ${BINARY} installed to ${dest}/${BINARY}"

  echo
  if on_path "$dest"; then
    echo "Next step:"
    echo "  linko init"
  else
    add_to_profiles "$dest"
    echo
    echo "Next steps:"
    echo "  exec \$SHELL       # reload your shell once"
    echo "  linko init"
  fi
}

main "$@"
