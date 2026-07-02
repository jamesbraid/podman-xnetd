# xnetd

Attaches rootless podman containers to rootful netavark bridges (dual-stack, IPv4 + IPv6) via podman's libnetwork socket API.

**License:** Apache-2.0  
**Copyright:** © 2026 James Braid  
**Language:** Go 1.24+  
**Build:** Static (`CGO_ENABLED=0`), no build tags, `-trimpath`

## What It Does

xnetd is a daemon that runs on the rootful container host and listens for attach/detach requests from rootless podman containers. When invoked via OCI hooks, it:

1. Receives a network namespace file descriptor from a rootless container's init process
2. Calls netavark to attach that container's network namespace to a rootful bridge network
3. Returns IP addresses and DNS configuration to the container's resolv.conf
4. Announces the container's IPs to the network via neighbor discovery (ARP for IPv4, NS for IPv6)
5. On container exit, detaches the network and releases IPAM leases

This enables rootless containers to appear as full peers on the rootful network — not isolated behind NAT.

## Annotations

Containers specify which networks to attach via podman annotations:

- **`org.octanix.rootful_networks`** (required): CSV of network names  
  Example: `"ovn0,management"`

- **`org.octanix.static_ip.<network>`** (optional): CSV of IPv4/IPv6 addresses for a specific network  
  Example: `org.octanix.static_ip.ovn0=10.0.0.5,fd00::5`

- **`org.octanix.container_name`** (optional): DNS name for the container (written to /etc/hosts in the container)

## Installation

### 1. Get the Binaries

Download from [releases](https://github.com/jamesbraid/xnetd/releases):

```bash
# Unpack the release tarball
tar -xzf xnetd_v1.0.0_linux_amd64.tar.gz -C /tmp

# Install binaries
sudo install -D -m 0755 /tmp/xnetd /usr/local/bin/xnetd
sudo install -D -m 0755 /tmp/oci-hook /usr/local/lib/xnet/oci-hook
```

### 2. Install OCI Hooks

```bash
# Hook definitions tell podman to run oci-hook on container lifecycle events
sudo install -D -m 0644 deploy/oci-hook.json /etc/containers/oci/hooks.d/xnetd-createruntime.json
sudo install -D -m 0644 deploy/oci-hook-poststop.json /etc/containers/oci/hooks.d/xnetd-poststop.json
```

### 3. Install Systemd Units

```bash
# Socket activation for xnetd daemon
sudo install -D -m 0644 deploy/xnetd.socket /etc/systemd/system/xnetd.socket
sudo install -D -m 0644 deploy/xnetd.service /etc/systemd/system/xnetd.service

# Reload systemd and enable
sudo systemctl daemon-reload
sudo systemctl enable xnetd.socket
```

### 4. Configure xnetd

Create `/etc/xnetd/config.toml`:

```toml
# Users allowed to request network attachment
allowed_users = ["myuser"]

[paths]
# Paths to podman network drivers
netavark = "/usr/lib/podman/netavark"
aardvark = "/usr/lib/podman/aardvark-dns"

[libnetwork]
# Rootful podman's network config and run directories
network_config_dir = "/etc/containers/networks"
network_run_dir = "/run/containers/networks"

[runtime]
# Where xnetd stores attachment state
state_dir = "/run/xnetd"
# Unix socket for OCI hooks to communicate with xnetd
socket = "/run/xnetd/sock"
```

### 5. Configure Rootless Podman

In `~/.config/containers/containers.conf` (or `/etc/containers/containers.conf`):

```toml
[containers]
# Point to xnetd's OCI hooks
hooks_dir = ["/etc/containers/oci/hooks.d"]
```

## Host Requirements

- **podman** (v4.0+): Container runtime
- **netavark** (v1.0+): Network driver with bridge support
- **aardvark-dns** (v1.0+): Embedded DNS
- **xnetd**: This daemon

All components must run on the rootful host. xnetd itself typically runs under a dedicated user (e.g., `_xnetd`) with CAP_NET_ADMIN and CAP_NET_RAW.

## Usage

Once installed and configured, simply attach annotations to your containers:

```bash
podman run \
  --annotation org.octanix.rootful_networks=mynet \
  --annotation org.octanix.container_name=app.local \
  --annotation org.octanix.static_ip.mynet=10.0.0.42 \
  myimage:latest
```

The xnetd daemon handles the rest. Logs appear in journalctl:

```bash
journalctl -u xnetd -f
```

## Development

Clone the repo and build:

```bash
git clone https://github.com/jamesbraid/xnetd.git
cd xnetd
go build -o xnetd ./cmd/xnetd
go build -o oci-hook ./cmd/oci-hook
```

Run unit tests:

```bash
go test ./...
```

Run integration tests (requires podman + netavark + systemd cgroup):

```bash
docker run --privileged --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup \
  -v "$PWD/out:/out:ro" \
  xnetd-harness:local
```

See `test/harness/` for the test setup.
