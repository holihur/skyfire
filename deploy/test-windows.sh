#!/usr/bin/env bash
#
# Smoketests the cross-compiled Windows binary under Wine on a Linux CI
# runner. The daemon runs in -demo mode (mock driver), which needs no Wintun
# driver, so the full login → session → API path is exercisable on Linux.
#
set -euo pipefail

BIN="${1:-/tmp/skyfired.exe}"
WINE="${WINE:-wine}"
ADDR="127.0.0.1:51999"
COOKIE="$(mktemp)"

if ! command -v "$WINE" >/dev/null 2>&1; then
  echo "SKIP: $WINE not available"
  exit 0
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "SKIP: curl not available"
  exit 0
fi

"$WINE" "$BIN" -demo -addr "$ADDR" &>"$BIN.log" &
PID=$!
cleanup() {
  kill "$PID" 2>/dev/null || true
  "$WINE" server -k 2>/dev/null || true
}
trap cleanup EXIT

# wait for the daemon to come up (wine startup is slow)
up=false
for _ in $(seq 1 40); do
  if curl -fsS "http://$ADDR/" >/dev/null 2>&1; then up=true; break; fi
  sleep 1
done
if [ "$up" != true ]; then
  echo "FAIL: daemon did not come up under wine"
  cat "$BIN.log" 2>/dev/null || true
  exit 1
fi

echo "== root page =="
curl -s -o /dev/null -w "root=%{http_code}\n" "http://$ADDR/"
echo "== login flow =="
curl -s -c "$COOKIE" -o /dev/null -w "login=%{http_code}\n" \
  -d '{"username":"admin","password":"demo"}' "http://$ADDR/api/login"
curl -s -o /dev/null -w "no-cookie=%{http_code}\n" "http://$ADDR/api/interfaces"
curl -s -b "$COOKIE" -o /dev/null -w "authed=%{http_code}\n" "http://$ADDR/api/interfaces"
curl -s -b "$COOKIE" -o /dev/null -w "logout=%{http_code}\n" -X POST "http://$ADDR/api/logout"
curl -s -b "$COOKIE" -o /dev/null -w "after-logout=%{http_code}\n" "http://$ADDR/api/health"

echo "OK: windows binary passed smoke test"