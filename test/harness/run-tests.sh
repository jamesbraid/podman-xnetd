#!/usr/bin/env bash
# Harness entrypoint: install canonical artifacts, configure xnetd, run suites.
# Runs as root inside the booted container.
set -Eeuo pipefail

log()  { printf '=== %s\n' "$*" >&2; }
trap 'log "FAILED at line $LINENO"' ERR

# Wait for systemd to finish booting.
until systemctl is-system-running --wait >/dev/null 2>&1 || [ "$(systemctl is-system-running 2>/dev/null)" = degraded ]; do
    sleep 1
done
log "systemd: $(systemctl is-system-running 2>/dev/null)"

# Install binaries and deploy artifacts.
install -m 0755 /out/xnetd              /usr/local/bin/xnetd
install -m 0755 /out/oci-hook           /usr/local/lib/xnet/oci-hook
install -m 0644 /out/dist/oci-hook.json /out/dist/oci-hook-poststop.json /etc/containers/oci/hooks.d/
install -m 0644 /out/dist/xnetd.service /out/dist/xnetd.socket /etc/systemd/system/

# Write config.toml.
cat > /etc/xnetd/config.toml <<'TOML'
# Harness has only the media user; do not list users that don't exist
# (xnetd fails to start if it can't resolve every allowed_users entry).
allowed_users = ["media"]
[paths]
netavark = "/usr/lib/podman/netavark"
aardvark = "/usr/lib/podman/aardvark-dns"
[libnetwork]
network_config_dir = "/etc/containers/networks"
network_run_dir    = "/run/containers/networks"
[runtime]
state_dir = "/run/xnetd"
TOML

systemctl daemon-reload
systemctl enable --now xnetd.socket
systemctl is-active xnetd.socket
test -S /run/xnetd/sock
log "xnetd.socket active, sock exists"

# Ensure media user session is running.
loginctl enable-linger media
until [ -d /run/user/1001 ]; do sleep 1; done
log "media user session ready"

RC=0
/usr/local/lib/xnetd-tests/integration/run.sh || RC=$?
/usr/local/lib/xnetd-tests/pressure/run.sh    || RC=$?
log "integration+pressure exit=$RC"
exit "$RC"
