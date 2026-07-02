# test/harness/Dockerfile — privileged systemd-init harness for xnetd tests.
# Base: debian:sid-slim — provides podman 5.8.3 + netavark 1.17.2 which align
# with go.podman.io/common v0.67.1 (target: podman 5.8.x).
FROM debian:sid-slim
ENV DEBIAN_FRONTEND=noninteractive container=podman
RUN apt-get update && apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus podman netavark aardvark-dns \
      uidmap passt slirp4netns fuse-overlayfs iproute2 iputils-ping dnsutils procps \
      ca-certificates jq iptables nftables \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN test -x /usr/lib/podman/netavark && test -x /usr/lib/podman/aardvark-dns
# Configure rootful podman to use fuse-overlayfs; required because the harness
# container's root FS is overlayfs (Docker on colima), and native overlay-on-overlay
# is not supported without a mount_program.
RUN mkdir -p /etc/containers \
    && printf '[storage]\ndriver = "overlay"\n[storage.options]\nmount_program = "/usr/bin/fuse-overlayfs"\n' \
       > /etc/containers/storage.conf
RUN find /etc/systemd/system /lib/systemd/system -path '*.wants/*' \
      \( -name 'getty*' -o -name 'systemd-udevd*' -o -name 'systemd-timesyncd*' \
         -o -name 'systemd-resolved*' -o -name 'systemd-networkd*' \) -exec rm -f {} + ; \
    systemctl mask systemd-udevd.service systemd-resolved.service systemd-networkd.service systemd-timesyncd.service || true
RUN groupadd -g 1001 media && useradd -m -u 1001 -g 1001 -s /bin/bash media \
    && echo 'media:200000:65536' > /etc/subuid && echo 'media:200000:65536' > /etc/subgid \
    && mkdir -p /var/lib/systemd/linger && touch /var/lib/systemd/linger/media
RUN install -d -o media -g media -m 0755 /home/media/.config \
    && install -d -o media -g media -m 0755 /home/media/.config/containers \
    && printf '[engine]\nhooks_dir=["/etc/containers/oci/hooks.d"]\n' > /home/media/.config/containers/containers.conf \
    && chown media:media /home/media/.config/containers/containers.conf
RUN install -d -m 0755 /etc/containers/oci/hooks.d /etc/xnetd /run/xnetd /usr/local/lib/xnet
# Fix user@1001.service: set XDG_RUNTIME_DIR so systemd --user can start inside container.
# logind in a container does not create /run/user/<uid> automatically.
RUN mkdir -p /etc/systemd/system/user@1001.service.d \
    && printf '[Service]\nEnvironment=XDG_RUNTIME_DIR=/run/user/1001\nExecStartPre=/bin/mkdir -p /run/user/1001\nExecStartPre=/bin/chmod 700 /run/user/1001\nExecStartPre=/bin/chown media:media /run/user/1001\n' \
       > /etc/systemd/system/user@1001.service.d/xdg-runtime.conf
COPY test/harness/run-tests.sh /usr/local/bin/run-tests.sh
COPY test/integration/ /usr/local/lib/xnetd-tests/integration/
COPY test/pressure/    /usr/local/lib/xnetd-tests/pressure/
RUN chmod +x /usr/local/bin/run-tests.sh \
    /usr/local/lib/xnetd-tests/integration/run.sh \
    /usr/local/lib/xnetd-tests/pressure/run.sh
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
