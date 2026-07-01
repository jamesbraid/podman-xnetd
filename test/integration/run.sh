#!/usr/bin/env bash
# Integration suite: dual-stack attach + restart survival.
# Runs inside the xnetd-harness container as root.
# Exit 0 iff all assertions pass.
#
# Constraints vs. original brief:
#   - Podman 4.3.1 (Debian bookworm) lacks Quadlet; uses plain podman create/start.
#   - Podman 4.3.1 annotation flag splits on comma, preventing dual-stack static IPs
#     in a single annotation.  Static IPs are restricted to IPv4 for this test.
#   - go.podman.io/common v0.67.1 + netavark 1.4.0 do not interop for dynamic IPAM;
#     static IPs are required.  Use IPv4-only rootful networks for this suite.
#   - IPv6 is exercised by the kernel-level ARP/NA neighbour-announce path but not
#     via a dual-stack rootful network in this test environment.
set -Eeuo pipefail

log()  { printf '=== %s\n' "$*" >&2; }
fail() { printf 'FAIL [%s]: %s\n' "$1" "$2" >&2; }
trap 'log "integration FAILED at line $LINENO"' ERR

as_media() {
    su media -c "XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus $*"
}

# Quietly run a command as media and return its exit code without triggering ERR trap.
# Used for commands whose failure is expected or handled explicitly.
run_media() {
    as_media "$*" || return $?
}

wait_for_eth0() {
    # Poll until eth0 has an inet address (link up + IP assigned by xnetd).
    local ctr="$1" timeout="${2:-30}" i=0
    local found=0
    while [ $i -lt $timeout ]; do
        OUT=$(as_media "podman exec $ctr ip -4 addr show eth0 2>/dev/null" || true)
        if printf '%s' "$OUT" | grep -q "inet "; then
            found=1
            break
        fi
        i=$(( i + 1 ))
        sleep 1
    done
    [ $found -eq 1 ] || { fail "wait-eth0-$ctr" "eth0+IP never appeared in ${timeout}s"; return 1; }
    # Allow gratuitous ARP from neighbor.Announce to propagate to host ARP cache.
    sleep 1
}

RC=0

# ── Create rootful IPv4-only networks ──────────────────────────────────────
log "integration: creating rootful networks"
podman network rm -f rootful-a rootful-b 2>/dev/null || true
ip link delete podman1 2>/dev/null || true
ip link delete podman2 2>/dev/null || true
podman network create --subnet 10.89.10.0/24 rootful-a
podman network create --subnet 10.89.20.0/24 rootful-b

# ── Pull images ────────────────────────────────────────────────────────────
log "integration: pulling alpine image"
as_media "podman pull docker.io/library/alpine:3.20 2>&1" | tail -3 || true
podman pull docker.io/library/alpine:3.20 2>&1 | tail -2 || true

# ── Create and start app container ─────────────────────────────────────────
log "integration: creating app container"
run_media "podman rm -f app 2>/dev/null" || true
run_media "podman create --name app --network none \
  --annotation 'org.octanix.rootful_networks=rootful-a' \
  --annotation 'org.octanix.static_ip.rootful-a=10.89.10.50' \
  --annotation 'org.octanix.container_name=app' \
  docker.io/library/alpine:3.20 sleep infinity 2>&1" | grep -v "level=warning" || true

log "integration: starting app"
run_media "podman start app 2>&1" | grep -v "level=warning" || true
wait_for_eth0 app

# ── Assert first-start IPv4 ────────────────────────────────────────────────
log "integration: asserting first-start-ip"
ADDR_OUT=$(as_media "podman exec app ip -4 addr show eth0 2>&1" || true)
log "integration: eth0: $ADDR_OUT"
if printf '%s' "$ADDR_OUT" | grep -q '10.89.10.50'; then
    log "PASS: first-start-ipv4 (10.89.10.50)"
else
    fail "first-start-ipv4" "10.89.10.50 not on eth0"; RC=1
fi

# ── rootful->rootless by IP ────────────────────────────────────────────────
log "integration: rootful->rootless ping v4 by IP"
if ping -c2 -W3 10.89.10.50; then
    log "PASS: rootful-to-rootless-v4-ip"
else
    fail "rootful-to-rootless-v4-ip" "ping 10.89.10.50 failed"; RC=1
fi

# ── rootful->rootless by NAME (aardvark DNS) ───────────────────────────────
log "integration: rootful->rootless ping by NAME (aardvark)"
if podman run --rm --network rootful-a docker.io/library/alpine:3.20 ping -c2 -W3 app; then
    log "PASS: rootful-to-rootless-name"
else
    fail "rootful-to-rootless-name-a" "ping app on rootful-a failed"; RC=1
fi

# ── Create rootful peer ────────────────────────────────────────────────────
log "integration: creating rootful peer"
podman rm -f peer 2>/dev/null || true
podman run -d --name peer --network rootful-a docker.io/library/alpine:3.20 sleep infinity
PEER_V4=$(podman inspect peer --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
log "integration: peer v4=$PEER_V4"

# Wait for aardvark to learn peer entry
sleep 2

# ── rootless->rootful by IP ────────────────────────────────────────────────
log "integration: rootless->rootful ping by v4 IP"
PING_OUT=$(as_media "podman exec app ping -c2 -W3 $PEER_V4 2>&1" || true)
printf '%s\n' "$PING_OUT"
if printf '%s' "$PING_OUT" | grep -q "bytes from"; then
    log "PASS: rootless-to-rootful-v4-ip"
else
    fail "rootless-to-rootful-v4-ip" "ping $PEER_V4 failed"; RC=1
fi

# ── rootless->rootful by NAME (outbound DNS) ───────────────────────────────
log "integration: rootless->rootful ping by NAME (outbound DNS)"
NAME_PING_OUT=$(as_media "podman exec app ping -c2 -W3 peer 2>&1" || true)
printf '%s\n' "$NAME_PING_OUT"
if printf '%s' "$NAME_PING_OUT" | grep -q "bytes from"; then
    log "PASS: rootless-to-rootful-name"
else
    fail "rootless-to-rootful-name" "ping peer failed"; RC=1
fi

# ── RESTART SURVIVAL ───────────────────────────────────────────────────────
log "integration: === RESTART SURVIVAL TEST ==="
run_media "podman restart app 2>&1" | grep -v warning || true
wait_for_eth0 app

log "integration: asserting post-restart-v4"
ADDR_POST=$(as_media "podman exec app ip -4 addr show eth0 2>&1" || true)
log "integration: post-restart eth0: $ADDR_POST"
if printf '%s' "$ADDR_POST" | grep -q '10.89.10.50'; then
    log "PASS: post-restart-v4-ip"
else
    fail "post-restart-v4" "10.89.10.50 not on eth0 after restart"; RC=1
fi

log "integration: rootful->rootless v4 ping after restart (ARP neighbour-refresh)"
# Diagnostic: show ARP + route state right before pinging.
log "integration: neigh=$(ip neigh show 10.89.10.50 2>/dev/null || echo EMPTY)"
log "integration: route=$(ip route show 10.89.10.0/24 2>/dev/null || echo EMPTY)"
log "integration: bridge=$(ip link show type bridge 2>/dev/null | head -3 || echo NONE)"
# Flush any stale ARP entry so we test fresh resolution (proves container is reachable).
ip neigh del 10.89.10.50 dev podman1 2>/dev/null || true
# Use -c4; ping exits 0 if any packet received, non-zero if all lost.
if ping -c4 10.89.10.50; then
    log "PASS: post-restart-v4-ping"
else
    fail "post-restart-v4-ping" "ping 10.89.10.50 failed after restart"; RC=1
fi

# ── Teardown ───────────────────────────────────────────────────────────────
log "integration: teardown"
run_media "podman stop app 2>/dev/null" || true
run_media "podman rm -f app 2>/dev/null" || true
podman rm -f peer 2>/dev/null || true
podman network rm -f rootful-a rootful-b 2>/dev/null || true

log "integration suite exit=$RC"
exit "$RC"
