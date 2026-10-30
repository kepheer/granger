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
systemctl disable --now granger-sni.service 2>/dev/null || true
while ip rule del fwmark 0x80000000/0x80000000 table 19845 priority 100 2>/dev/null; do :; done
ip route flush table 19845 2>/dev/null || true

cleanup_linked_configs() {
  directory="$1"
  service="$2"
  for conf in "$directory"/*.conf; do
    [ -L "$conf" ] || continue
    target="$(readlink -f "$conf" 2>/dev/null || true)"
    case "$target" in
      /etc/granger/*)
        name="$(basename "$conf" .conf)"
        if [ "$service" = wg-quick ] && [ "$name" = wg0 ] && [ "$REMOVE_ADMIN_WG" != true ]; then
          cp -L "$conf" "$conf.granger-preserved"
          chmod 0600 "$conf.granger-preserved"
          mv -f "$conf.granger-preserved" "$conf"
          continue
        fi
        systemctl disable --now "$service@$name.service" 2>/dev/null || true
        rm -f "$conf"
        ;;
    esac
  done
}

cleanup_linked_configs /etc/wireguard wg-quick
cleanup_linked_configs /etc/amnezia/amneziawg awg-quick
cleanup_linked_configs /etc/openvpn/server openvpn-server
cleanup_linked_configs /etc/openvpn/client openvpn-client

for unit in /etc/systemd/system/granger-*.socket /etc/systemd/system/granger-*.timer /etc/systemd/system/granger-*.service; do
  [ -f "$unit" ] || continue
  systemctl disable --now "$(basename "$unit")" 2>/dev/null || true
  rm -f "$unit"
done

if command -v nft >/dev/null 2>&1; then
  for handle in $(nft -a list chain inet host_security input 2>/dev/null | awk '/jump granger_control/ {print $NF}'); do
    nft delete rule inet host_security input handle "$handle" 2>/dev/null || true
  done
  nft flush chain inet host_security granger_control 2>/dev/null || true
  nft delete chain inet host_security granger_control 2>/dev/null || true
fi

if command -v ip6tables >/dev/null 2>&1; then
  while ip6tables -D OUTPUT -j GRANGER_USER_OUTPUT 2>/dev/null; do :; done
  ip6tables -F GRANGER_USER_OUTPUT 2>/dev/null || true
  ip6tables -X GRANGER_USER_OUTPUT 2>/dev/null || true
fi

if [ -r /var/lib/granger/route-tables ]; then
  while IFS= read -r table; do
    case "$table" in
      ''|*[!0-9]*) continue ;;
    esac
    while ip rule del table "$table" 2>/dev/null; do :; done
    ip route flush table "$table" 2>/dev/null || true
  done </var/lib/granger/route-tables
fi

iptables -D FORWARD -j GRANGER_USER_FORWARD 2>/dev/null || true
iptables -F GRANGER_USER_FORWARD 2>/dev/null || true
iptables -X GRANGER_USER_FORWARD 2>/dev/null || true
while iptables -D INPUT -j GRANGER_USER_INPUT 2>/dev/null; do :; done
iptables -F GRANGER_USER_INPUT 2>/dev/null || true
iptables -X GRANGER_USER_INPUT 2>/dev/null || true
iptables -D OUTPUT -j GRANGER_USER_OUTPUT 2>/dev/null || true
iptables -F GRANGER_USER_OUTPUT 2>/dev/null || true
iptables -X GRANGER_USER_OUTPUT 2>/dev/null || true
iptables -t mangle -D OUTPUT -j GRANGER_OUTPUT 2>/dev/null || true
iptables -t mangle -F GRANGER_OUTPUT 2>/dev/null || true
iptables -t mangle -X GRANGER_OUTPUT 2>/dev/null || true
while iptables -t nat -D PREROUTING -j GRANGER_DNS 2>/dev/null; do :; done
iptables -t nat -F GRANGER_DNS 2>/dev/null || true
iptables -t nat -X GRANGER_DNS 2>/dev/null || true
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
rm -f /usr/local/sbin/granger /usr/local/libexec/granger-wait-network
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
