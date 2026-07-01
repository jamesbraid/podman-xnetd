#!/usr/bin/env bash
# Shared helpers for the pressure suite.
# Source this file; do not execute directly.

count_netns() { ls -1 /run/xnetd/netns 2>/dev/null | wc -l; }
count_veth()  { ip -o link | grep -c veth || true; }
count_fd()    { ls -1 /proc/$(pgrep -x xnetd)/fd 2>/dev/null | wc -l; }

as_media() {
    su media -c "XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus $*"
}

wait_for_eth0() {
    local ctr="$1" timeout="${2:-30}" i=0
    # Wait until eth0 has an inet address (not just the link).
    while ! as_media "podman exec $ctr ip -4 addr show eth0 2>/dev/null" | grep -q "inet "; do
        i=$(( i + 1 ))
        [ $i -ge $timeout ] && return 1
        sleep 1
    done
    sleep 1  # allow gratuitous ARP to propagate
}
