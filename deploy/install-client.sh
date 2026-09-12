#!/usr/bin/env bash
#
# Skyfire desktop client one-click installer (Linux / macOS).
#
#   curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install-client.sh | sudo bash
#
# Downloads the prebuilt client for the current platform, installs it to
# PREFIX/bin, and installs the runtime dependencies it needs on Linux
# (iproute2, and checks for a DNS backend). macOS needs nothing extra.
#
# Env:
#   SKYFIRE_VERSION=vX.Y.Z   pin a release tag (default: latest)
#   PREFIX=/usr/local        install prefix
#
# Windows: use deploy/install-client.ps1 instead.
#
set -euo pipefail

REPO="holihur/skyfire"
PREFIX="${PREFIX:-/usr/local}"
VERSION="${SKYFIRE_VERSION:-latest}"

info() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mWARN:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

detect() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux|darwin) ;;
    *) die "unsupported OS: $os (on Windows use deploy/install-client.ps1)" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported architecture: $arch" ;;
  esac
  DEST="$PREFIX/bin/skyfire-client"
}

install_deps() {
  if [ "$os" != "linux" ]; then
    info "macOS: no extra dependencies required"
    return 0
  fi
  local missing=()
  command -v ip >/dev/null 2>&1 || missing+=("iproute2")
  if ! command -v resolvectl >/dev/null 2>&1 && ! command -v resolvconf >/dev/null 2>&1; then
    warn "no resolvectl/resolvconf found; the client will edit /etc/resolv.conf directly (backed up)"
  fi
  [ ${#missing[@]} -eq 0 ] && { info "linux dependencies satisfied"; return 0; }
  info "installing missing dependencies: ${missing[*]}"
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq && apt-get install -y "${missing[@]}"
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y "${missing[@]}"
  elif command -v yum >/dev/null 2>&1; then
    yum install -y "${missing[@]}"
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache "${missing[@]}"
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm "${missing[@]}"
  else
    warn "could not install ${missing[*]} automatically; install it manually"
  fi
}

main() {
  detect
  [ "$(id -u)" -eq 0 ] || die "run as root (sudo): the client needs CAP_NET_ADMIN to create the TUN device"

  if [ "$VERSION" = "latest" ]; then
    ASSET_BASE="https://github.com/$REPO/releases/latest/download"
  else
    ASSET_BASE="https://github.com/$REPO/releases/download/$VERSION"
  fi

  command -v curl >/dev/null 2>&1 || die "curl is required"
  install_deps

  info "installing skyfire-client $VERSION ($os/$arch)"
  install -d "$PREFIX/bin"

  URL="$ASSET_BASE/skyfire-client-$os-$arch"
  info "downloading $URL"
  curl -fSL --retry 3 --connect-timeout 15 -o "$DEST.tmp" "$URL" \
    || die "download failed for $VERSION ($os-$arch): pass SKYFIRE_VERSION=vX.Y.Z to pick a specific version"
  chmod +x "$DEST.tmp"

  if command -v sha256sum >/dev/null 2>&1; then
    SUM="sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    SUM="shasum -a 256"
  fi
  if [ -n "${SUM:-}" ]; then
    info "verifying checksum"
    if curl -fsSL --retry 3 --connect-timeout 15 -o "$DEST.tmp.checksums" "$ASSET_BASE/checksums.txt"; then
      want="$(grep "skyfire-client-$os-$arch\$" "$DEST.tmp.checksums" | awk '{print $1}')"
      got="$($SUM "$DEST.tmp" | awk '{print $1}')"
      rm -f "$DEST.tmp.checksums"
      if [ -z "$want" ] || [ "$want" != "$got" ]; then
        rm -f "$DEST.tmp"
        die "checksum mismatch: expected ${want:-<missing>}, got $got"
      fi
    else
      warn "checksum file unavailable; skipping verification"
    fi
  else
    warn "no sha256 tool found; skipping checksum verification"
  fi

  mv -f "$DEST.tmp" "$DEST"
  info "installed: $DEST"

  if [ "$os" = "darwin" ]; then
    # curl downloads are not quarantined, but clear the bit just in case.
    xattr -d com.apple.quarantine "$DEST" 2>/dev/null || true
  fi

  echo
  echo "Get the connect URL from the Web UI (Peer details -> Desktop client), then:"
  echo "  sudo skyfire-client -cli -connect '<connect-url>'"
}

main "$@"
