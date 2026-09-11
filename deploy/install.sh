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
  install -d "$PREFIX/bin" "$ETC"

  URL="https://github.com/$REPO/releases/download/$VERSION/skyfired-$os-$arch$BIN_EXT"
  info "downloading $URL"
  curl -fSL --retry 3 -o "$DEST.tmp" "$URL" \
    || die "download failed for $VERSION ($os/$arch): run 'curl -fsSL https://raw.githubusercontent.com/$REPO/main/deploy/install.sh | sudo sudo SKYFIRE_VERSION=vX.Y.Z bash' to pick a different version"
  chmod +x "$DEST.tmp"
  mv -f "$DEST.tmp" "$DEST"
  info "binary installed at $DEST"

  if [[ "$os" != "linux" || ! -d /run/systemd/system ]]; then
    info "done. Run:  $DEST -demo   (mock mode, no privileges)"
    info "real tunnels need Wintun (Windows) / CAP_NET_ADMIN; configure with -config <path>"
    return 0
  fi

  if [[ "$(id -u)" -ne 0 ]]; then
    info "non-root: binary installed; systemd unit skipped (run: sudo bash $0)"
    return 0
  fi

  cat > /etc/systemd/system/skyfire.service <<EOF
[Unit]
Description=Skyfire WireGuard management daemon
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