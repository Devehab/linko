#!/usr/bin/env bash
#
# linko installer
#
#   curl -fsSL https://raw.githubusercontent.com/ibtkrgo/linko/main/install.sh | bash
#
# Environment variables:
#   LINKO_VERSION   version to install (default: latest)
#   LINKO_INSTALL   install directory  (default: /usr/local/bin, or ~/.local/bin)

set -euo pipefail

REPO="${LINKO_REPO:-ibtkrgo/linko}"
VERSION="${LINKO_VERSION:-latest}"
BINARY="linko"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
dim()   { printf '\033[2m%s\033[0m\n' "$*"; }

die() { red "error: $*" >&2; exit 1; }

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

choose_install_dir() {
  if [ -n "${LINKO_INSTALL:-}" ]; then
    echo "$LINKO_INSTALL"
  elif [ -w /usr/local/bin ] 2>/dev/null; then
    echo /usr/local/bin
  elif [ "$(id -u)" = "0" ]; then
    echo /usr/local/bin
  else
    echo "$HOME/.local/bin"
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

  local platform version tag archive url tmp dest
  platform="$(detect_platform)"
  tag="$(resolve_version)"
  version="${tag#v}"

  archive="${BINARY}_${version}_${platform}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${tag}/${archive}"

  dim "Installing ${BINARY} ${tag} (${platform})"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  curl -fsSL "$url" -o "$tmp/$archive" \
    || die "download failed: $url"
  tar -xzf "$tmp/$archive" -C "$tmp"
  [ -f "$tmp/$BINARY" ] || die "the archive does not contain a ${BINARY} binary"
  chmod +x "$tmp/$BINARY"

  dest="$(choose_install_dir)"
  mkdir -p "$dest"

  if [ -w "$dest" ]; then
    mv "$tmp/$BINARY" "$dest/$BINARY"
  else
    dim "elevating with sudo to write to $dest"
    sudo mv "$tmp/$BINARY" "$dest/$BINARY"
  fi

  green "✓ ${BINARY} installed to ${dest}/${BINARY}"

  if ! command -v "$BINARY" >/dev/null 2>&1; then
    echo
    red "! ${dest} is not on your PATH"
    echo "  Add this to your shell profile:"
    echo "    export PATH=\"${dest}:\$PATH\""
  fi

  echo
  echo "Next step:"
  echo "  linko init"
}

main "$@"
