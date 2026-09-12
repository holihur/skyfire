#!/usr/bin/env bash
#
# Gateway watchdog for a safe full-tunnel connect test (Linux).
#
# The catastrophic failure mode of a full tunnel without an endpoint exception
# is a routing loop: ALL traffic (including LAN) gets pushed into the tunnel,
# so the SSH session drops and the box can only be recovered by a reboot.
#
# This script pings the LAN gateway every few seconds. If the gateway becomes
# unreachable it runs recover.sh immediately, restoring normal routing.
#
# Run as root, fully detached:  sudo setsid scripts/watchdog.sh </dev/null >/dev/null 2>&1 &
#
set -u

GW="${SKYFIRE_WATCH_GW:-192.168.2.1}"
RECOVER="${SKYFIRE_RECOVER:-$(cd "$(dirname "$0")" && pwd)/recover.sh}"
INTERVAL="${SKYFIRE_WATCH_INTERVAL:-5}"
MAX_FAILS="${SKYFIRE_WATCH_MAX_FAILS:-3}"
LOG="${SKYFIRE_WATCH_LOG:-/tmp/skyfire-watchdog.log}"

exec >>"$LOG" 2>&1
echo "=== watchdog start $(date) gw=$GW interval=${INTERVAL}s maxfails=$MAX_FAILS ==="

fails=0
while true; do
  if ping -c1 -W2 "$GW" >/dev/null 2>&1; then
    fails=0
  else
    fails=$((fails + 1))
    echo "$(date) ping $GW failed ($fails/$MAX_FAILS)"
  fi

  if [ "$fails" -ge "$MAX_FAILS" ]; then
    echo "$(date) gateway unreachable -> running $RECOVER"
    "$RECOVER"
    echo "$(date) recovery triggered; watchdog exiting"
    exit 0
  fi

  sleep "$INTERVAL"
done
