#!/bin/sh
set -eu

REMOVE_ADMIN_WG=false
if [ "${1:-}" = "--remove-admin-wireguard" ]; then
  REMOVE_ADMIN_WG=true
fi

if [ "$(id -u)" -ne 0 ]; then
  printf 'error: run as root\n' >&2
  exit 1
fi

systemctl disable --now granger.service 2>/dev/null || true

if [ -r /var/lib/granger/route-tables ]; then
  while IFS= read -r table; do
    case "$table" in
      ''|*[!0-9]*) continue ;;
    esac
    while ip rule del table "$table" 2>/dev/null; do :; done
    ip route flush table "$table" 2>/dev/null || true
  done </var/lib/granger/route-tables
fi

iptables -t mangle -D PREROUTING -j GRANGER_PREROUTING 2>/dev/null || true
iptables -D FORWARD -j GRANGER_FORWARD 2>/dev/null || true
iptables -t nat -D POSTROUTING -j GRANGER_POSTROUTING 2>/dev/null || true
iptables -t mangle -F GRANGER_PREROUTING 2>/dev/null || true
iptables -F GRANGER_FORWARD 2>/dev/null || true
iptables -t nat -F GRANGER_POSTROUTING 2>/dev/null || true
iptables -t mangle -X GRANGER_PREROUTING 2>/dev/null || true
iptables -X GRANGER_FORWARD 2>/dev/null || true
iptables -t nat -X GRANGER_POSTROUTING 2>/dev/null || true

if command -v ipset >/dev/null 2>&1; then
  ipset list -name 2>/dev/null | while IFS= read -r set_name; do
    case "$set_name" in
      granger_*) ipset destroy "$set_name" 2>/dev/null || true ;;
    esac
  done
fi

rm -f /etc/systemd/system/granger.service
rm -f /etc/dnsmasq.d/granger.conf
rm -f /usr/local/sbin/granger
rm -rf /etc/granger /opt/granger /var/lib/granger /var/log/granger

if [ "$REMOVE_ADMIN_WG" = true ]; then
  systemctl disable --now wg-quick@wg0.service 2>/dev/null || true
  if [ -L /etc/wireguard/wg0.conf ]; then
    rm -f /etc/wireguard/wg0.conf
  fi
  rm -f /root/granger-admin.conf
fi

systemctl daemon-reload 2>/dev/null || true
systemctl restart dnsmasq.service 2>/dev/null || true

printf 'Granger-owned artifacts removed.\n'
if [ "$REMOVE_ADMIN_WG" != true ]; then
  printf 'Admin WireGuard files were preserved. Rerun with --remove-admin-wireguard to remove the Granger wg0 symlink and /root/granger-admin.conf.\n'
fi
