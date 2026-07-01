#!/usr/bin/env bash
# Pressure suite: leak, concurrency, kill-mid-attach, clean-failure.
# Runs inside the xnetd-harness container as root.
# Exit 0 iff no resource growth beyond tolerance and all cases behave.
#
# Constraints vs. original brief:
#   - Podman 4.3.1 (Debian bookworm) lacks Quadlet; uses plain podman create/start.
#   - Annotation comma-split bug prevents dual-stack static IPs; v4-only networks.
#   - netavark 1.4.0 requires static IPs; each container needs a distinct annotation.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

log()  { printf '=== %s\n' "$*" >&2; }
fail() { printf 'FAIL [%s]: %s\n' "$1" "$2" >&2; }
trap 'log "pressure FAILED at line $LINENO"' ERR

RC=0

# ── Setup: one IPv4-only network ───────────────────────────────────────────
log "pressure: setup network"
podman network rm -f press-net 2>/dev/null || true
podman network create --subnet 10.89.30.0/24 press-net

log "pressure: pulling alpine image"
as_media "podman pull docker.io/library/alpine:3.20 2>&1" | tail -3
podman pull docker.io/library/alpine:3.20 2>&1 | tail -2

# ── Helper: create+start a named container on press-net ───────────────────
# Usage: start_press_ctr <name> <ip>
start_press_ctr() {
    local name="$1" ip="$2"
    as_media "podman rm -f $name 2>/dev/null; true"
    as_media "podman create --name $name --network none \
        --annotation 'org.octanix.rootful_networks=press-net' \
        --annotation 'org.octanix.static_ip.press-net=$ip' \
        --annotation 'org.octanix.container_name=$name' \
        docker.io/library/alpine:3.20 sleep infinity 2>&1" | grep -v warning || true
    as_media "podman start $name 2>&1" | grep -v warning || true
}

# wait_for_eth0 in lib.sh uses plain podman exec (runs as root, works for any container)
# Override here to run as media since the container is rootless:
wait_for_eth0_ctr() {
    local ctr="$1" timeout="${2:-30}" i=0
    while ! as_media "podman exec $ctr ip link show eth0 2>/dev/null" | grep -q eth0; do
        i=$(( i + 1 ))
        [ $i -ge $timeout ] && { fail "wait-eth0-$ctr" "eth0 never appeared in ${timeout}s"; return 1; }
        sleep 1
    done
}

# ── Test 1: restart-churn leak (25 cycles) ────────────────────────────────
log "pressure: TEST 1 — restart-churn leak (25 cycles)"
start_press_ctr churn 10.89.30.100
wait_for_eth0_ctr churn

BASE_NETNS=$(count_netns)
BASE_VETH=$(count_veth)
BASE_FD=$(count_fd)
log "pressure: baseline netns=$BASE_NETNS veth=$BASE_VETH fd=$BASE_FD"

for i in $(seq 1 25); do
    as_media "podman restart churn 2>&1" | grep -v warning || true
    wait_for_eth0_ctr churn 30
done
sleep 5

FINAL_NETNS=$(count_netns)
FINAL_VETH=$(count_veth)
FINAL_FD=$(count_fd)
log "pressure: after 25 restarts: netns=$FINAL_NETNS veth=$FINAL_VETH fd=$FINAL_FD"

[ "$FINAL_NETNS" -le $(( BASE_NETNS + 1 )) ] \
    || { fail "churn-netns-leak" "netns: $BASE_NETNS -> $FINAL_NETNS (want ≤$(( BASE_NETNS + 1 )))"; RC=1; }
[ "$FINAL_VETH" -le $(( BASE_VETH + 2 )) ] \
    || { fail "churn-veth-leak" "veth: $BASE_VETH -> $FINAL_VETH (want ≤$(( BASE_VETH + 2 )))"; RC=1; }
[ "$FINAL_FD" -le $(( BASE_FD + 4 )) ] \
    || { fail "churn-fd-leak" "fd: $BASE_FD -> $FINAL_FD (want ≤$(( BASE_FD + 4 )))"; RC=1; }
log "PASS: churn-leak (netns=$FINAL_NETNS veth=$FINAL_VETH fd=$FINAL_FD)"

as_media "podman stop churn 2>/dev/null" || true
as_media "podman rm -f churn 2>/dev/null" || true
sleep 2

# ── Test 2: concurrent attach (10 containers) ─────────────────────────────
log "pressure: TEST 2 — concurrent attach (10 containers)"

# Assign static IPs .10 through .19
PIDS=()
for i in $(seq 1 10); do
    IP="10.89.30.$((9 + i))"
    (start_press_ctr "concurrent${i}" "$IP") &
    PIDS+=($!)
done
for pid in "${PIDS[@]}"; do wait "$pid" || true; done

sleep 5

log "pressure: checking concurrent containers each have a distinct IP"
SEEN_IPS=()
CONCURRENT_RC=0
for i in $(seq 1 10); do
    IP=$(as_media "podman exec concurrent${i} ip -4 addr show eth0 2>/dev/null" \
         | grep -oP '(?<=inet )[0-9.]+' | head -1 || echo "MISSING")
    log "pressure: concurrent${i} ip=$IP"
    if [ "$IP" = "MISSING" ]; then
        fail "concurrent-missing-ip-${i}" "concurrent${i} has no IP" || true; RC=1; CONCURRENT_RC=1
    else
        for seen in "${SEEN_IPS[@]:-}"; do
            if [ "$seen" = "$IP" ]; then
                fail "concurrent-duplicate-ip" "IP $IP duplicated"; RC=1; CONCURRENT_RC=1; break
            fi
        done
        SEEN_IPS+=("$IP")
    fi
done

XNETD_ERR=$(journalctl -u xnetd --no-pager -q 2>/dev/null | grep -ci 'panic' || true)
[ "$XNETD_ERR" -eq 0 ] \
    || { fail "concurrent-xnetd-panics" "xnetd logged $XNETD_ERR panic lines"; RC=1; }
[ "$CONCURRENT_RC" -eq 0 ] && log "PASS: concurrent-attach"

# Stop concurrent containers
for i in $(seq 1 10); do
    as_media "podman stop concurrent${i} 2>/dev/null" || true
    as_media "podman rm -f concurrent${i} 2>/dev/null" || true
done
sleep 2

# ── Test 3: kill-mid-attach ────────────────────────────────────────────────
log "pressure: TEST 3 — kill-mid-attach"
PRE_VETH=$(count_veth)

# Start a container in the background; kill xnetd immediately after hook fires.
# The race is intentional — we want to test that socket-activation respawns.
(start_press_ctr "killtest" "10.89.30.200") &
SPID=$!
sleep 0.7
XNETD_PID=$(pgrep -x xnetd 2>/dev/null || true)
if [ -n "$XNETD_PID" ]; then
    kill -9 "$XNETD_PID"
    log "pressure: killed xnetd pid=$XNETD_PID"
else
    log "pressure: xnetd not running at kill time (hook already returned)"
fi
wait "$SPID" || true
as_media "podman rm -f killtest 2>/dev/null" || true
sleep 3

systemctl is-active xnetd.socket \
    || { fail "kill-socket-inactive" "xnetd.socket not active after kill"; RC=1; }

# Fresh attach should work (daemon respawns on next connection)
start_press_ctr "killtest2" "10.89.30.201"
wait_for_eth0_ctr "killtest2"

POST_VETH=$(count_veth)
log "pressure: veth before=$PRE_VETH after kill+fresh-start=$POST_VETH"
[ "$POST_VETH" -le $(( PRE_VETH + 4 )) ] \
    || { fail "kill-orphan-veth" "veth grew from $PRE_VETH to $POST_VETH"; RC=1; }
log "PASS: kill-mid-attach"

as_media "podman stop killtest2 2>/dev/null" || true
as_media "podman rm -f killtest2 2>/dev/null" || true
sleep 2

# ── Test 4: nonexistent network (clean failure, no leak) ──────────────────
log "pressure: TEST 4 — nonexistent network"
PRE_VETH4=$(count_veth)
PRE_NETNS4=$(count_netns)

as_media "podman rm -f badnet 2>/dev/null; true"
as_media "podman create --name badnet --network none \
    --annotation 'org.octanix.rootful_networks=does-not-exist' \
    --annotation 'org.octanix.container_name=badnet' \
    docker.io/library/alpine:3.20 sleep infinity 2>&1" | grep -v warning || true

# Expected to fail: use || true so the ERR trap does not fire.
# The container will fail to start (OCI hook returns non-zero) or
# immediately exit.  The real assertion is the state check below.
BADNET_OUT=$(as_media "podman start badnet 2>&1" || true)
log "pressure: badnet start output: ${BADNET_OUT:-<empty>}"
sleep 2
BADNET_STATE=$(as_media "podman inspect --format '{{.State.Status}}' badnet 2>/dev/null" || echo "missing")
log "pressure: badnet state=$BADNET_STATE"
[ "$BADNET_STATE" != "running" ] \
    || { fail "badnet-should-not-run" "badnet is still running with a nonexistent network"; RC=1; }

systemctl is-active xnetd.socket \
    || { fail "badnet-socket-dead" "xnetd.socket not active after bad-network attempt"; RC=1; }

POST_VETH4=$(count_veth)
POST_NETNS4=$(count_netns)
[ "$POST_VETH4" -le $(( PRE_VETH4 + 1 )) ] \
    || { fail "badnet-veth-leak" "veth: $PRE_VETH4 -> $POST_VETH4"; RC=1; }
[ "$POST_NETNS4" -le $(( PRE_NETNS4 + 1 )) ] \
    || { fail "badnet-netns-leak" "netns: $PRE_NETNS4 -> $POST_NETNS4"; RC=1; }

BADNET_LOG=$(journalctl -u xnetd --no-pager -q 2>/dev/null \
    | grep -i 'does-not-exist\|not found\|unknown network\|no such network' | tail -5 || true)
log "pressure: xnetd error log for bad network: ${BADNET_LOG:-<none found>}"
log "PASS: nonexistent-network"

as_media "podman rm -f badnet 2>/dev/null" || true

# ── Teardown ───────────────────────────────────────────────────────────────
log "pressure: teardown"
podman network rm -f press-net 2>/dev/null || true

log "pressure suite exit=$RC"
exit "$RC"
