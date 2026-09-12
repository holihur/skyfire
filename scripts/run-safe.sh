#!/usr/bin/env bash
#
# Run `skyfire-client -connect ...` with a safety net so a broken full-tunnel
# connect can never leave the box needing a reboot.
#
# Three independent layers, none of which depends on the client behaving:
#   1. watchdog.sh    - pings the LAN gateway and recovers if it goes away.
#   2. hard fallback  - recovers after RUN_SECS+30 seconds no matter what.
#   3. `timeout -k`   - SIGTERMs (then SIGKILLs) the client after RUN_SECS.
#
# Usage (as root, because the client needs TUN + route privileges):
#   sudo scripts/run-safe.sh '<connect-url>' [run-seconds]
#
set -u

CONNECT="${1:-}"
RUN_SECS="${2:-60}"

if [ -z "$CONNECT" ]; then
  echo "usage: $0 '<connect-url>' [run-seconds]" >&2
  exit 2
fi
if [ "$(id -u)" -ne 0 ]; then
  echo "error: run as root (sudo), the client needs CAP_NET_ADMIN" >&2
  exit 2
fi

HERE="$(cd "$(dirname "$0")" && pwd)"
BIN="${SKYFIRE_CLIENT_BIN:-$HERE/../skyfire-client}"
RECOVER="$HERE/recover.sh"
WATCHDOG="$HERE/watchdog.sh"

# Pick the default gateway as the watchdog target unless overridden.
GW="${SKYFIRE_WATCH_GW:-$(ip route show default 2>/dev/null | awk '{print $3; exit}')}"
: "${GW:?could not determine default gateway; set SKYFIRE_WATCH_GW}"

SNAP="${SKYFIRE_NET_SNAP:-/tmp/skyfire-net-before.txt}"
rm -f "$SNAP" 2>/dev/null || true
echo "== snapshot (before) -> $SNAP =="
{ echo "route:"; ip route; echo "rule:"; ip rule; echo "link:"; ip -br link; echo "addr:"; ip -br addr; } \
  > "$SNAP"

echo "== layer 1: watchdog (gw=$GW) =="
setsid "$WATCHDOG" </dev/null >/dev/null 2>&1 &

echo "== layer 2: hard fallback (recover in $((RUN_SECS + 30))s) =="
setsid bash -c "sleep $((RUN_SECS + 30)); '$RECOVER'" </dev/null >/dev/null 2>&1 &

echo "== layer 3: run client for ${RUN_SECS}s (then timeout) =="
timeout -k 10 "$RUN_SECS" "$BIN" -cli -connect "$CONNECT"
rc=$?
echo "== client exited rc=$rc =="

# Let the client's own teardown finish, then check what's left.
sleep 2
echo "== verify =="
ip route | grep -E 'skyfire|table 51821' || echo "no skyfire routes in main"
ip rule   | grep -E '51821|suppress_prefixlength' || echo "no skyfire rules"

# Stop the watchdog if it is still running (hard fallback is idempotent).
pkill -f "$WATCHDOG" 2>/dev/null || true

exit "$rc"
