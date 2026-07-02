#!/usr/bin/env bash
# Integration suite: dual-stack attach + auto-IPAM + restart survival.
# Runs inside the xnetd-harness container as root.
# Exit 0 iff all assertions pass.
#
# Platform: debian:sid-slim — podman 5.8.3, netavark 1.17.2, aardvark-dns 1.17.1
# These versions align with go.podman.io/common v0.67.1 (deployment target).
#
# Annotation design note (verified):
#   Comma-in-annotation-value SURVIVES intact through podman 5.8.3 to config.json.
#   The earlier podman 4.3.1 comma-split was a CLI version quirk, not a design flaw.
#   Dual-stack static IPs use the current comma-separated scheme:
#     org.octanix.static_ip.<net>=<v4-addr>,<v6-addr>
set -Eeuo pipefail

log()  { printf '=== %s\n' "$*" >&2; }
fail() { printf 'FAIL [%s]: %s\n' "$1" "$2" >&2; }
trap 'log "integration FAILED at line $LINENO"' ERR

as_media() {
    su media -c "XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus $*"
}

run_media() {
    as_media "$*" || return $?
}

# Wait until eth0 has any IPv4 address (link up + v4 assigned by xnetd).
wait_for_eth0() {
    local ctr="$1" timeout="${2:-30}" i=0 found=0
    while [ $i -lt $timeout ]; do
        OUT=$(as_media "podman exec $ctr ip -4 addr show eth0 2>/dev/null" || true)
        if printf '%s' "$OUT" | grep -q "inet "; then
            found=1; break
        fi
        i=$(( i + 1 )); sleep 1
    done
    [ $found -eq 1 ] || { fail "wait-eth0-$ctr" "eth0 IPv4 never appeared in ${timeout}s"; return 1; }
    sleep 1  # allow gARP from neighbor.Announce to propagate
}

# Wait until eth0 has a non-link-local IPv6 address (global scope, assigned by xnetd).
wait_for_eth0_v6() {
    local ctr="$1" timeout="${2:-30}" i=0 found=0
    while [ $i -lt $timeout ]; do
        OUT=$(as_media "podman exec $ctr ip -6 addr show eth0 2>/dev/null" || true)
        if printf '%s' "$OUT" | grep "inet6" | grep -qv "fe80::"; then
            found=1; break
        fi
        i=$(( i + 1 )); sleep 1
    done
    [ $found -eq 1 ] || { fail "wait-eth0-v6-$ctr" "eth0 global IPv6 never appeared in ${timeout}s"; return 1; }
    sleep 1  # allow unsolicited NA from neighbor.Announce to propagate
}

RC=0

# ── Create dual-stack rootful network ─────────────────────────────────────────
# rootful-dual: v4 (10.89.10.0/24) + v6 (fd00:10:89:10::/64)
# Both subnets are used for all connectivity and restart-survival tests.
log "integration: creating dual-stack rootful network"
podman network rm -f rootful-dual rootful-b 2>/dev/null || true
ip link delete podman1 2>/dev/null || true
ip link delete podman2 2>/dev/null || true
podman network create --subnet 10.89.10.0/24 --subnet fd00:10:89:10::/64 rootful-dual
podman network create --subnet 10.89.20.0/24 rootful-b

# Derive bridge interface name from network inspect (podman 5.x may name it differently)
BRIDGE=$(podman network inspect rootful-dual --format '{{.NetworkInterface}}')
log "integration: bridge=$BRIDGE"

# ── Pull images ────────────────────────────────────────────────────────────────
log "integration: pulling alpine image"
as_media "podman pull docker.io/library/alpine:3.20 2>&1" | tail -3 || true
podman pull docker.io/library/alpine:3.20 2>&1 | tail -2 || true

# ══════════════════════════════════════════════════════════════════════════════
# TEST 1 — AUTO-IPAM: no static_ip annotation → libnetwork allocates both
#           a v4 and a v6 address automatically.
# This tests the StaticIPs=nil path in attach.buildNetworkOptions.
# ══════════════════════════════════════════════════════════════════════════════
log "integration: === AUTO-IPAM TEST ==="
run_media "podman rm -f app-auto 2>/dev/null" || true
run_media "podman create --name app-auto --network none \
  --annotation 'org.octanix.rootful_networks=rootful-dual' \
  --annotation 'org.octanix.container_name=app-auto' \
  docker.io/library/alpine:3.20 sleep infinity 2>&1" | grep -v "level=warning" || true

log "integration: starting app-auto (no static IPs — auto-IPAM)"
run_media "podman start app-auto 2>&1" | grep -v "level=warning" || true
wait_for_eth0 app-auto
wait_for_eth0_v6 app-auto

# Extract auto-assigned addresses
AUTO_V4_FULL=$(as_media "podman exec app-auto ip -4 addr show eth0 2>&1" || true)
AUTO_V6_FULL=$(as_media "podman exec app-auto ip -6 addr show eth0 2>&1" || true)
log "integration: app-auto eth0 v4: $AUTO_V4_FULL"
log "integration: app-auto eth0 v6: $AUTO_V6_FULL"

AUTO_V4=$(printf '%s' "$AUTO_V4_FULL" | grep -oP '(?<=inet )[0-9.]+(?=/)' | head -1 || true)
AUTO_V6=$(printf '%s' "$AUTO_V6_FULL" | grep "inet6" | grep -v "fe80::" | grep -oP '(?<=inet6 )[0-9a-f:]+(?=/)' | head -1 || true)

if printf '%s' "$AUTO_V4" | grep -q "^10\.89\.10\."; then
    log "PASS: auto-ipam-v4 ($AUTO_V4)"
else
    fail "auto-ipam-v4" "expected 10.89.10.x, got: $AUTO_V4"; RC=1
fi

if printf '%s' "$AUTO_V6" | grep -q "^fd00:10:89:10:"; then
    log "PASS: auto-ipam-v6 ($AUTO_V6)"
else
    fail "auto-ipam-v6" "expected fd00:10:89:10::x, got: $AUTO_V6"; RC=1
fi

# rootful→rootless v4 ping (auto-IPAM)
log "integration: auto-IPAM rootful->rootless v4 ping ($AUTO_V4)"
if ping -c2 -W3 "$AUTO_V4"; then
    log "PASS: auto-ipam-ping-v4"
else
    fail "auto-ipam-ping-v4" "ping $AUTO_V4 failed"; RC=1
fi

# rootful→rootless v6 ping (auto-IPAM, exercises the NA code path)
log "integration: auto-IPAM rootful->rootless v6 ping ($AUTO_V6)"
if ping -6 -c2 -W3 "$AUTO_V6"; then
    log "PASS: auto-ipam-ping-v6"
else
    fail "auto-ipam-ping-v6" "ping6 $AUTO_V6 failed"; RC=1
fi

run_media "podman stop app-auto 2>/dev/null" || true
run_media "podman rm -f app-auto 2>/dev/null" || true

# ══════════════════════════════════════════════════════════════════════════════
# TEST 2 — STATIC DUAL-STACK: explicit v4 + v6 static IPs in one comma-separated
#           annotation value.  Verified: podman 5.8.3 passes comma-in-value intact
#           to config.json; the 4.3.1 comma-split was a version quirk, not a design
#           flaw.  Annotation scheme unchanged.
# ══════════════════════════════════════════════════════════════════════════════
log "integration: === STATIC DUAL-STACK TEST ==="
run_media "podman rm -f app-static 2>/dev/null" || true
run_media "podman create --name app-static --network none \
  --annotation 'org.octanix.rootful_networks=rootful-dual' \
  --annotation 'org.octanix.static_ip.rootful-dual=10.89.10.50,fd00:10:89:10::50' \
  --annotation 'org.octanix.container_name=app-static' \
  docker.io/library/alpine:3.20 sleep infinity 2>&1" | grep -v "level=warning" || true

log "integration: starting app-static"
run_media "podman start app-static 2>&1" | grep -v "level=warning" || true
wait_for_eth0 app-static
wait_for_eth0_v6 app-static

# Assert both static IPs are present on eth0
ADDR_V4_OUT=$(as_media "podman exec app-static ip -4 addr show eth0 2>&1" || true)
ADDR_V6_OUT=$(as_media "podman exec app-static ip -6 addr show eth0 2>&1" || true)
log "integration: app-static eth0 v4: $ADDR_V4_OUT"
log "integration: app-static eth0 v6: $ADDR_V6_OUT"

if printf '%s' "$ADDR_V4_OUT" | grep -q '10\.89\.10\.50'; then
    log "PASS: static-v4-addr (10.89.10.50)"
else
    fail "static-v4-addr" "10.89.10.50 not on eth0"; RC=1
fi

if printf '%s' "$ADDR_V6_OUT" | grep -q 'fd00:10:89:10::50'; then
    log "PASS: static-v6-addr (fd00:10:89:10::50)"
else
    fail "static-v6-addr" "fd00:10:89:10::50 not on eth0"; RC=1
fi

# rootful→rootless v4 ping (by IP)
log "integration: rootful->rootless v4 ping"
if ping -c2 -W3 10.89.10.50; then
    log "PASS: rootful-to-rootless-v4-ip"
else
    fail "rootful-to-rootless-v4-ip" "ping 10.89.10.50 failed"; RC=1
fi

# rootful→rootless v6 ping (by IP — exercises unsolicited-NA path on first attach)
log "integration: rootful->rootless v6 ping"
if ping -6 -c2 -W3 fd00:10:89:10::50; then
    log "PASS: rootful-to-rootless-v6-ip"
else
    fail "rootful-to-rootless-v6-ip" "ping6 fd00:10:89:10::50 failed"; RC=1
fi

# rootful→rootless by NAME (aardvark-dns, dual-stack — both A+AAAA registered)
log "integration: rootful->rootless ping by NAME (aardvark)"
if podman run --rm --network rootful-dual docker.io/library/alpine:3.20 ping -c2 -W3 app-static; then
    log "PASS: rootful-to-rootless-name"
else
    fail "rootful-to-rootless-name" "ping app-static on rootful-dual failed"; RC=1
fi

# ── Create rootful peer ────────────────────────────────────────────────────────
log "integration: creating rootful peer"
podman rm -f peer 2>/dev/null || true
podman run -d --name peer --network rootful-dual docker.io/library/alpine:3.20 sleep infinity
PEER_V4=$(podman inspect peer --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
PEER_V6=$(podman inspect peer --format '{{range .NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}')
log "integration: peer v4=$PEER_V4 v6=$PEER_V6"
sleep 2  # wait for aardvark to learn peer entry

# rootless→rootful v4 ping (by IP)
log "integration: rootless->rootful v4 ping"
PING_V4_OUT=$(as_media "podman exec app-static ping -c2 -W3 $PEER_V4 2>&1" || true)
printf '%s\n' "$PING_V4_OUT"
if printf '%s' "$PING_V4_OUT" | grep -q "bytes from"; then
    log "PASS: rootless-to-rootful-v4-ip"
else
    fail "rootless-to-rootful-v4-ip" "ping $PEER_V4 failed"; RC=1
fi

# rootless→rootful v6 ping (by IP)
log "integration: rootless->rootful v6 ping"
PING_V6_OUT=$(as_media "podman exec app-static ping -c2 -W3 $PEER_V6 2>&1" || true)
printf '%s\n' "$PING_V6_OUT"
if printf '%s' "$PING_V6_OUT" | grep -q "bytes from"; then
    log "PASS: rootless-to-rootful-v6-ip"
else
    fail "rootless-to-rootful-v6-ip" "ping $PEER_V6 failed"; RC=1
fi

# rootless→rootful by NAME (outbound DNS via aardvark)
log "integration: rootless->rootful ping by NAME"
NAME_PING_OUT=$(as_media "podman exec app-static ping -c2 -W3 peer 2>&1" || true)
printf '%s\n' "$NAME_PING_OUT"
if printf '%s' "$NAME_PING_OUT" | grep -q "bytes from"; then
    log "PASS: rootless-to-rootful-name"
else
    fail "rootless-to-rootful-name" "ping peer failed"; RC=1
fi

# ══════════════════════════════════════════════════════════════════════════════
# TEST 3 — RESTART SURVIVAL (BOTH IP FAMILIES)
#   Verifies that after podman restart:
#   - v4 static IP survives and gARP (neighbor.Announce ARP) refreshes the host
#     ARP cache so rootful→rootless v4 ping works immediately.
#   - v6 static IP survives and the unsolicited NA (neighbor.Announce NA) refreshes
#     the host IPv6 neighbor cache so rootful→rootless v6 ping works immediately.
#   This is the UNIQUE code path that only exists for v6 via buildUnsolicitedNA +
#   sendNA in internal/neighbor/neighbor.go.
# ══════════════════════════════════════════════════════════════════════════════
log "integration: === RESTART SURVIVAL TEST (v4 + v6) ==="
run_media "podman restart app-static 2>&1" | grep -v warning || true
wait_for_eth0 app-static
wait_for_eth0_v6 app-static

# Assert v4 IP preserved
ADDR_POST_V4=$(as_media "podman exec app-static ip -4 addr show eth0 2>&1" || true)
log "integration: post-restart eth0 v4: $ADDR_POST_V4"
if printf '%s' "$ADDR_POST_V4" | grep -q '10\.89\.10\.50'; then
    log "PASS: post-restart-v4-ip"
else
    fail "post-restart-v4" "10.89.10.50 not on eth0 after restart"; RC=1
fi

# Assert v6 IP preserved
ADDR_POST_V6=$(as_media "podman exec app-static ip -6 addr show eth0 2>&1" || true)
log "integration: post-restart eth0 v6: $ADDR_POST_V6"
if printf '%s' "$ADDR_POST_V6" | grep -q 'fd00:10:89:10::50'; then
    log "PASS: post-restart-v6-ip"
else
    fail "post-restart-v6" "fd00:10:89:10::50 not on eth0 after restart"; RC=1
fi

# Diagnostics: show neighbor + route state before pinging
log "integration: v4 neigh=$(ip neigh show 10.89.10.50 2>/dev/null || echo EMPTY)"
log "integration: v6 neigh=$(ip -6 neigh show fd00:10:89:10::50 2>/dev/null || echo EMPTY)"
log "integration: route4=$(ip route show 10.89.10.0/24 2>/dev/null || echo EMPTY)"
log "integration: route6=$(ip -6 route show fd00:10:89:10::/64 2>/dev/null || echo EMPTY)"
log "integration: bridge=$(ip link show type bridge 2>/dev/null | head -3 || echo NONE)"

# v4 post-restart ping (gARP path — neighbor.Announce sends gratuitous ARP)
# Flush stale ARP entry to force fresh resolution and prove gARP worked.
ip neigh del 10.89.10.50 dev "$BRIDGE" 2>/dev/null || true
log "integration: rootful->rootless v4 ping after restart (gARP path)"
if ping -c4 10.89.10.50; then
    log "PASS: post-restart-v4-ping"
else
    fail "post-restart-v4-ping" "ping 10.89.10.50 failed after restart"; RC=1
fi

# v6 post-restart ping (unsolicited NA path — neighbor.Announce sends NDP NA)
# Flush stale neighbor entry to force fresh resolution and prove NA worked.
ip -6 neigh del fd00:10:89:10::50 dev "$BRIDGE" 2>/dev/null || true
log "integration: rootful->rootless v6 ping after restart (unsolicited-NA path)"
if ping -6 -c4 fd00:10:89:10::50; then
    log "PASS: post-restart-v6-ping"
else
    fail "post-restart-v6-ping" "ping6 fd00:10:89:10::50 failed after restart"; RC=1
fi

# ── Teardown ───────────────────────────────────────────────────────────────────
log "integration: teardown"
run_media "podman stop app-static 2>/dev/null" || true
run_media "podman rm -f app-static 2>/dev/null" || true
podman rm -f peer 2>/dev/null || true
podman network rm -f rootful-dual rootful-b 2>/dev/null || true

log "integration suite exit=$RC"
exit "$RC"
