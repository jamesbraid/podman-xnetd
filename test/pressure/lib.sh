#!/usr/bin/env bash
# Shared helpers for the pressure suite.
# Source this file; do not execute directly.

count_netns() { ls -1 /run/xnetd/netns 2>/dev/null | wc -l; }
count_veth()  { ip -o link | grep -c veth || true; }
count_fd()    { ls -1 /proc/$(pgrep -x xnetd)/fd 2>/dev/null | wc -l; }

as_media() {
    su media -c "XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus $*"
}

# Wait until eth0 has any IPv4 address.
wait_for_eth0() {
    local ctr="$1" timeout="${2:-30}" i=0
    while ! as_media "podman exec $ctr ip -4 addr show eth0 2>/dev/null" | grep -q "inet "; do
        i=$(( i + 1 ))
        [ $i -ge $timeout ] && return 1
        sleep 1
    done
    sleep 1  # allow gratuitous ARP to propagate
}

# Wait until eth0 has a non-link-local IPv6 address (global scope).
wait_for_eth0_v6() {
    local ctr="$1" timeout="${2:-30}" i=0
    while true; do
        OUT=$(as_media "podman exec $ctr ip -6 addr show eth0 2>/dev/null" || true)
        if printf '%s' "$OUT" | grep "inet6" | grep -qv "fe80::"; then
            break
        fi
        i=$(( i + 1 ))
        [ $i -ge $timeout ] && return 1
        sleep 1
    done
    sleep 1  # allow unsolicited NA to propagate
}
