#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR: run this script as root." >&2
  exit 1
fi

if command -v modprobe >/dev/null 2>&1; then
  modprobe tun >/dev/null 2>&1 || true
fi

if [ ! -d /dev/net ]; then
  mkdir -p /dev/net
fi

if [ ! -c /dev/net/tun ]; then
  if ! mknod /dev/net/tun c 10 200; then
    cat >&2 <<'EOF'
ERROR: cannot create /dev/net/tun on this host.

This host is probably a restricted LXC/container. WireGuard-in-Docker needs
the host to expose a TUN device. On a Proxmox LXC host, enable TUN for the
container, for example:

  lxc.cgroup2.devices.allow: c 10:200 rwm
  lxc.mount.entry: /dev/net/tun dev/net/tun none bind,create=file

Then restart this test server and run docker/host-prep.sh again.
EOF
    exit 1
  fi
fi

chmod 0666 /dev/net/tun

cat >/etc/sysctl.d/99-proidentity-docker.conf <<'EOF'
net.ipv4.ip_forward=1
net.ipv4.conf.all.src_valid_mark=1
EOF

sysctl --system >/dev/null || true

echo "Docker WireGuard host prep complete."
