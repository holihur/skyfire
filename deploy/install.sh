#!/usr/bin/env bash
#
# Skyfire one-click binary installer.
#
#   curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh | sudo bash
#
# Downloads a prebuilt release binary for the current platform. No Go / Node
# toolchain required. Optionally installs a systemd unit on Linux.
#
# Env:
#   SKYFIRE_VERSION=vX.Y.Z   pin a release tag (default: latest)
#   PREFIX=/usr/local         install prefix
#   ETC=/etc/skyfire          config directory (Linux)
#
set -euo pipefail

REPO="holihur/skyfire"
PREFIX="${PREFIX:-/usr/local}"
ETC="${ETC:-/etc/skyfire}"
VERSION="${SKYFIRE_VERSION:-latest}"

info()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die()   { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

detect() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux)  BIN_EXT="" ;;
    darwin) BIN_EXT="" ;;
    msys*|mingw*|cygwin) os="windows"; BIN_EXT=".exe" ;;
    *) die "unsupported OS: $os" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported architecture: $arch" ;;
  esac
  DEST="$PREFIX/bin/skyfired$BIN_EXT"
}

main() {
  detect
  info "installing skyfired $VERSION ($os/$arch) into $PREFIX"

  command -v curl >/dev/null 2>&1 || die "curl is required"
  if [[ "$VERSION" == "latest" ]]; then
    ASSET_BASE="https://github.com/$REPO/releases/latest/download"
  else
    ASSET_BASE="https://github.com/$REPO/releases/download/$VERSION"
  fi
  install -d "$PREFIX/bin"

  URL="$ASSET_BASE/skyfired-$os-$arch$BIN_EXT"
  info "downloading $URL"
  curl -fSL --retry 3 --connect-timeout 15 -o "$DEST.tmp" "$URL" \
    || die "download failed for $VERSION ($os-$arch): rerun with SKYFIRE_VERSION=vX.Y.Z to pick a specific version"
  chmod +x "$DEST.tmp"

  if command -v sha256sum >/dev/null 2>&1; then
    SUM="sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    SUM="shasum -a 256"
  fi
  if [[ -n "${SUM:-}" ]]; then
    info "verifying checksum"
    SUMS_URL="$ASSET_BASE/checksums.txt"
    curl -fsSL --retry 3 --connect-timeout 15 -o "$DEST.tmp.checksums" "$SUMS_URL" \
      || die "checksum file download failed: $SUMS_URL"
    want="$(grep "skyfired-$os-$arch$BIN_EXT\$" "$DEST.tmp.checksums" | awk '{print $1}')"
    got="$($SUM "$DEST.tmp" | awk '{print $1}')"
    rm -f "$DEST.tmp.checksums"
    if [[ -z "$want" || "$want" != "$got" ]]; then
      rm -f "$DEST.tmp"
      die "checksum mismatch: expected ${want:-<missing>}, got $got"
    fi
  else
    info "WARNING: sha256 tool not found, skipping checksum verification"
  fi
  mv -f "$DEST.tmp" "$DEST"
  info "binary installed at $DEST"

  if [[ "$os" != "linux" || ! -d /run/systemd/system ]]; then
    info "done. Run:  $DEST -demo   (mock mode, no privileges)"
    info "real tunnels need Wintun (Windows) / CAP_NET_ADMIN; configure with -config <path>"
    return 0
  fi

  if [[ "$(id -u)" -ne 0 ]]; then
    info "non-root: binary installed; systemd unit skipped (rerun with sudo for the service)"
    return 0
  fi

  install -d "$ETC"
  cat > /etc/systemd/system/skyfire.service <<EOF
[Unit]
Description=Skyfire WireGuard management daemon
Documentation=https://github.com/holihur/skyfire
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$DEST -config $ETC/config.json -addr :51821
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=$ETC
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now skyfire
  info "service skyfire started on :51821"
  info "web login: http://<host>:51821  (username 'admin', password printed at first start in:)"
  journalctl -u skyfire --no-pager -n 60 2>/dev/null | grep -oE 'password=[A-Za-z0-9]+' | tail -1 || true
}

main "$@"