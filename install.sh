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
DOCS_URL="https://github.com/${REPO}/blob/main/GUIDE.md"

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

  # Return non-zero rather than dying: the caller falls back to building from
  # source, which covers platforms we do not ship binaries for.
  #
  # Git Bash, MSYS2 and Cygwin all report their own kernel name rather than
  # "windows", so without this the one-liner silently fell through to a source
  # build and only worked if Go happened to be installed.
  case "$os" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*|windows*) os="windows" ;;
    *) return 1 ;;
  esac

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) return 1 ;;
  esac

  echo "${os}_${arch}"
}

# How to install Go on this machine, in the user's own package manager.
go_install_hint() {
  case "$(uname -s)" in
    Darwin)
      echo "    brew install go"
      echo "    # no Homebrew? download the .pkg from https://go.dev/dl/"
      ;;
    Linux)
      echo "    sudo apt install golang-go      # Debian / Ubuntu"
      echo "    sudo dnf install golang         # Fedora"
      echo "    sudo pacman -S go               # Arch"
      echo "    # or download the tarball from https://go.dev/dl/"
      ;;
    *)
      echo "    https://go.dev/dl/"
      ;;
  esac
}

find_go() {
  command -v go >/dev/null 2>&1 && return 0
  # The official macOS/Linux packages install here without touching PATH.
  local c
  for c in /usr/local/go/bin /opt/homebrew/bin "$HOME/go/bin"; do
    if [ -x "$c/go" ]; then
      PATH="$c:$PATH"; export PATH
      dim "found go in $c (not on your PATH)"
      return 0
    fi
  done
  return 1
}

build_from_source() {
  local dest="$1" ref="${2:-$VERSION}"

  if ! find_go; then
    echo
    red "! There is no prebuilt binary for $(uname -s)/$(uname -m),"
    red "  and building from source needs Go, which is not installed."
    echo
    echo "  Install Go, then run this installer again:"
    go_install_hint
    echo
    exit 1
  fi

  dim "building from source with $(go version | awk '{print $3}')"
  # Stamp the version, otherwise `linko --version` reports "dev".
  GOBIN="$dest" go install -ldflags "-s -w -X main.version=${ref#v}" \
    "github.com/${REPO}@${ref}" || die "build from source failed"
}

# unpack <archive> <dir> — tar.gz everywhere, zip on Windows.
unpack() {
  case "$1" in
    *.zip)
      if command -v unzip >/dev/null 2>&1; then
        unzip -q -o "$1" -d "$2"
      elif tar -xf "$1" -C "$2" 2>/dev/null; then
        : # Windows 10+ ships bsdtar, which reads zip archives
      else
        die "extracting $1 needs unzip, or a tar that understands zip"
      fi
      ;;
    *)
      tar -xzf "$1" -C "$2" || die "could not unpack $1"
      ;;
  esac
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

  local platform version tag archive url dest exe ext
  dest="$(choose_install_dir)"
  mkdir -p "$dest"

  platform="$(detect_platform)" || platform=""

  exe="$BINARY"
  ext="tar.gz"
  case "$platform" in
    windows_*) exe="${BINARY}.exe"; ext="zip" ;;
  esac

  if [ -z "$platform" ]; then
    dim "no prebuilt binary for $(uname -s)/$(uname -m)"
    build_from_source "$dest"
  else
    tag="$(resolve_version)"
    version="${tag#v}"
    archive="${BINARY}_${version}_${platform}.${ext}"
    url="https://github.com/${REPO}/releases/download/${tag}/${archive}"

    dim "Installing ${BINARY} ${tag} (${platform})"

    TMP="$(mktemp -d)"

    if curl -fsSL "$url" -o "$TMP/$archive"; then
      unpack "$TMP/$archive" "$TMP"
      [ -f "$TMP/$exe" ] || die "the archive does not contain ${exe}"
      chmod +x "$TMP/$exe"

      if [ -w "$dest" ]; then
        mv "$TMP/$exe" "$dest/$exe"
      else
        dim "elevating with sudo to write to $dest"
        sudo mv "$TMP/$exe" "$dest/$exe"
      fi
    else
      dim "no release archive at $url — building from source instead"
      build_from_source "$dest" "$tag"
    fi
  fi

  [ -x "$dest/$exe" ] || die "installation failed: $dest/$exe is missing"

  # Prove the binary actually runs here. A wrong architecture or a libc
  # mismatch surfaces as "cannot execute binary file", which is far easier to
  # act on now than the first time the user types `linko`.
  if ! "$dest/$exe" --version >/dev/null 2>&1; then
    red "! ${BINARY} was installed but will not run on this system"
    echo "  $("$dest/$exe" --version 2>&1 | head -1)"
    echo
    echo "  Please open an issue with the output of:"
    echo "    uname -sm && ldd --version 2>&1 | head -1"
    echo "    https://github.com/${REPO}/issues"
    exit 1
  fi

  green "✓ ${BINARY} installed to ${dest}/${exe}"

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

  echo
  echo "Before that you need a Cloudflare API token with BOTH permissions:"
  echo "  Zone     ->  DNS                ->  Edit"
  echo "  Account  ->  Cloudflare Tunnel  ->  Edit"
  echo "  Create it at https://dash.cloudflare.com/profile/api-tokens"
  echo
  echo "Full guide (install, token, commands, troubleshooting):"
  green "  ${DOCS_URL}"
  echo "  or run:  linko docs"
}

main "$@"
