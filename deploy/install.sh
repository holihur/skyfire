#!/usr/bin/env bash
#
# Skyfire one-click install.
#
#   curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh | sudo bash
#
# Behavior:
#   * downloads the latest prebuilt release binary for this platform when
#     available (set SKYFIRE_VERSION=vX.Y.Z to pin a version), otherwise
#     builds from source (requires go >= 1.26, node >= 20, pnpm >= 9);
#   * installs the daemon + systemd unit;
#   * starts the service and prints the auto-generated web login password.
#
set -euo pipefail

REPO="holihur/skyfire"
PREFIX="${PREFIX:-/usr/local}"
ETC="${ETC:-/etc/skyfire}"
BIN="$PREFIX/bin/skyfired"
UNIT="/lib/systemd/system/skyfire.service"
VERSION="${SKYFIRE_VERSION:-}"

info()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die()   { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

if [[ "$(id -u)" -ne 0 ]]; then
  die "run as root (sudo)"
fi

detect_os_arch() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux) : ;;
    darwin) : ;;
    *) die "unsupported OS: $os (build from source manually instead)" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    armv7l) arch=armv7l ;;
    *) die "unsupported architecture: $arch" ;;
  esac
}

install_binary() {
  local dest="$1"

  # prefer a prebuilt release
  if command -v curl >/dev/null 2>&1 && attempt_release_download "$dest"; then
    return
  fi

  info "no prebuilt binary available; building from source"
  command -v go >/dev/null 2>&1 || die "go not found: install Go >= 1.26 or place a binary at $dest"
  command -v node >/dev/null 2>&1 || die "node not found: install Node >= 20"
  command -v pnpm >/dev/null 2>&1 || die "pnpm not found: install pnpm (corepack)"

  local src
  if [[ "${SKYFIRE_SOURCE:-}" != "" ]]; then
    src="$SKYFIRE_SOURCE"
  elif [[ -f Makefile && -d backend && -d frontend ]]; then
    src="$(pwd)"
  else
    src="$(mktemp -d)/skyfire"
    info "cloning $REPO"
    git clone --depth 1 "https://github.com/$REPO.git" "$src" \
      || die "clone failed (install git or pre-populate SKYFIRE_SOURCE)"
  fi

  ( cd "$src" && make all )
  install -m 0755 "$src/skyfired" "$dest"
}

attempt_release_download() {
  local dest="$1"
  local gh="${VERSION:-latest}"
  info "downloading $gh release for $os-$arch"
  local url
  url="https://github.com/$REPO/releases/download/$gh/skyfired-$os-$arch"
  if ! curl -fsSL -o "$dest" "$url"; then
    info "release binary not found ($gh / $os-$arch); will build from source"
    return 1
  fi
  chmod +x "$dest"
  return 0
}

main() {
  detect_os_arch
  info "installing for $os-$arch into $PREFIX"

  install -d "$PREFIX/bin" "$ETC"
  TMPBIN="$(mktemp)"
  install_binary "$TMPBIN"
  install -m 0755 "$TMPBIN" "$BIN"
  rm -f "$TMPBIN"

  # systemd unit (written inline so curl|bash always works)
  cat > "$UNIT" <<EOF
[Unit]
Description=Skyfire WireGuard management daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN -config $ETC/config.json -addr :51821
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

  systemctl daemon-reload || true
  systemctl enable --now skyfire || true
  systemctl restart skyfire || true

  sleep 1
  info "login credentials (also in the journal):"
  journalctl -u skyfire --no-pager -n 40 2>/dev/null | grep -oE 'password=[A-Za-z0-9]+' | tail -1 || true
  echo
  echo "  open http://<this-host>:51821 and sign in with username 'admin'"
}

main "$@"