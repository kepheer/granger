#!/usr/bin/env bash
set -Eeuo pipefail

VERSION="${1:?usage: package-deb.sh VERSION BINARY GUI_DIR OUT_DIR}"
BINARY="${2:?usage: package-deb.sh VERSION BINARY GUI_DIR OUT_DIR}"
GUI_DIR="${3:?usage: package-deb.sh VERSION BINARY GUI_DIR OUT_DIR}"
OUT_DIR="${4:?usage: package-deb.sh VERSION BINARY GUI_DIR OUT_DIR}"

PACKAGE="granger"
ARCH="amd64"
DEB_VERSION="${VERSION#v}"
WORK_DIR="$(mktemp -d)"
ROOT="${WORK_DIR}/${PACKAGE}_${DEB_VERSION}_${ARCH}"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

[ -x "$BINARY" ] || {
  printf 'binary is missing or not executable: %s\n' "$BINARY" >&2
  exit 1
}
[ -r "${GUI_DIR}/index.html" ] || {
  printf 'GUI build is missing: %s\n' "$GUI_DIR" >&2
  exit 1
}

mkdir -p \
  "${ROOT}/DEBIAN" \
  "${ROOT}/usr/local/sbin" \
  "${ROOT}/opt/granger/gui" \
  "${ROOT}/etc/default" \
  "${ROOT}/lib/systemd/system" \
  "${ROOT}/usr/share/doc/granger"

install -m 0755 "$BINARY" "${ROOT}/usr/local/sbin/granger"
cp -a "${GUI_DIR}/." "${ROOT}/opt/granger/gui/"
install -m 0644 LICENSE "${ROOT}/usr/share/doc/granger/copyright"

cat >"${ROOT}/etc/default/granger" <<'EOF'
GRANGER_LISTEN=10.19.84.51:1984
GRANGER_GUI_DIR=/opt/granger/gui
EOF

cat >"${ROOT}/lib/systemd/system/granger.service" <<'EOF'
[Unit]
Description=Granger private routing control plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-/etc/default/granger
ExecStart=/usr/local/sbin/granger serve-gui ${GRANGER_LISTEN} ${GRANGER_GUI_DIR}
Restart=on-failure
RestartSec=2s
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF

installed_size="$(du -sk "${ROOT}" | awk '{print $1}')"
cat >"${ROOT}/DEBIAN/control" <<EOF
Package: granger
Version: ${DEB_VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Installed-Size: ${installed_size}
Maintainer: Granger Maintainers <maintainers@granger.local>
Depends: ca-certificates, curl, jq, iproute2, nftables, iptables, dnsmasq, dnsutils, systemd, procps
Description: Private VPN routing control plane
 Granger is a self-hosted routing orchestrator for VPN entrypoints,
 upstreams, bypass rules, DNS, and runtime health.
EOF

cat >"${ROOT}/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e

CONFIG_PATH="/etc/granger/granger.yaml"

detect_value() {
  command="$1"
  fallback="$2"
  value="$(sh -c "$command" 2>/dev/null || true)"
  if [ -n "$value" ]; then
    printf '%s' "$value"
  else
    printf '%s' "$fallback"
  fi
}

mkdir -p /etc/granger/upstreams /etc/granger/outputs /etc/granger/secrets /var/lib/granger /var/log/granger /opt/granger
chmod 700 /etc/granger /etc/granger/upstreams /etc/granger/outputs /etc/granger/secrets

if [ ! -e "$CONFIG_PATH" ]; then
  uplink_if="$(detect_value "ip -4 route show default | awk '{print \$5; exit}'" "auto")"
  uplink_gw="$(detect_value "ip -4 route show default | awk '{print \$3; exit}'" "auto")"
  public_ip="$(detect_value "ip -4 route get 1.1.1.1 | awk '/src/ {for(i=1;i<=NF;i++) if(\$i==\"src\"){print \$(i+1); exit}}'" "auto")"
  cat >"$CONFIG_PATH" <<CFG
server:
  public_ip: "${public_ip}"
  uplink_if: "${uplink_if}"
  uplink_gw: "${uplink_gw}"
  dns_interface: "lo"
  dns_listen: "127.0.0.1"
  dns_upstreams:
    - "8.8.8.8"
    - "9.9.9.9"
users: {}
outputs: {}
upstreams: {}
rules: []
CFG
  chmod 600 "$CONFIG_PATH"
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable --now granger.service || true
fi
EOF

cat >"${ROOT}/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e

if [ "$1" = "remove" ] && command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now granger.service || true
fi
EOF

cat >"${ROOT}/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
EOF

chmod 0755 "${ROOT}/DEBIAN/postinst" "${ROOT}/DEBIAN/prerm" "${ROOT}/DEBIAN/postrm"

mkdir -p "$OUT_DIR"
dpkg-deb --root-owner-group --build "$ROOT" "${OUT_DIR}/granger_${DEB_VERSION}_${ARCH}.deb"
