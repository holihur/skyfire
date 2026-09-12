#!/usr/bin/env bash
#
# Skyfire client test recovery (Linux).
#
# Force-tears down any leftover tunnel state so a broken full-tunnel connect
# test never leaves the machine needing a reboot. Idempotent: safe to run any
# number of times, with or without the tunnel up.
#
# Run as root:  sudo scripts/recover.sh
#
set -u

TUN_NAME="${SKYFIRE_TUN:-skyfire}"
ROUTE_TABLE="${SKYFIRE_TABLE:-51821}"
LOG="${SKYFIRE_RECOVER_LOG:-/tmp/skyfire-recover.log}"

exec >>"$LOG" 2>&1
echo "=== recover start $(date) ==="

# 1. Stop the client so it can't re-add state after we tear down.
pkill -TERM -f 'skyfire-client' 2>/dev/null || true
sleep 1
pkill -KILL -f 'skyfire-client' 2>/dev/null || true

# 2. Delete the TUN device. The kernel removes any routes referencing it,
#    including the per-AllowedIPs routes the client added to the main table.
ip link del "$TUN_NAME" 2>/dev/null || true

# 3. Flush the policy-routing table used by full tunnels. The full-tunnel
#    default route lives here, never in the main table.
ip route flush table "$ROUTE_TABLE" 2>/dev/null || true
ip -6 route flush table "$ROUTE_TABLE" 2>/dev/null || true

# 4. Remove the policy rules the client adds. Match by content rather than
#    priority number, so this works regardless of the order `ip rule add`
#    happened to assign.
for fam in "" "-6"; do
  while read -r pref; do
    [ -n "$pref" ] || continue
    ip $fam rule del pref "$pref" 2>/dev/null || true
  done < <(ip $fam rule show 2>/dev/null \
    | awk '/'"$ROUTE_TABLE"'|suppress_prefixlength 0/{gsub(":","",$1); print $1}')
done

echo "=== recover done $(date) ==="
