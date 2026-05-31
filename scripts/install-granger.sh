#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="granger"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${GRANGER_BIN:-${SCRIPT_DIR}/granger}"
GUI_SRC="${GRANGER_GUI_SRC:-${SCRIPT_DIR}/dist/gui}"
BIN_DST="/usr/local/sbin/granger"
GUI_DST="/opt/granger/gui"
CONFIG_PATH="/etc/granger/granger.yaml"
SERVICE_PATH="/etc/systemd/system/granger.service"
LISTEN="${GRANGER_LISTEN:-10.19.84.51:1984}"

export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

info() {
  printf '\n==> %s\n' "$*"
}

need_root() {
  [ "$(id -u)" -eq 0 ] || die "run as root: sudo bash ./install-granger.sh"
}

detect_os() {
  [ -r /etc/os-release ] || die "/etc/os-release not found"
  # shellcheck disable=SC1091
  . /etc/os-release
  OS_ID="${ID:-}"
  OS_VERSION="${VERSION_ID:-}"
  ARCH="$(dpkg --print-architecture 2>/dev/null || uname -m)"

  case "$ARCH" in
    amd64|x86_64) ;;
    *) die "unsupported architecture: ${ARCH}. Granger MVP supports amd64 only." ;;
  esac

  case "$OS_ID:$OS_VERSION" in
    debian:12|debian:13|ubuntu:22.04|ubuntu:24.04) ;;
    *) die "unsupported OS: ${OS_ID:-unknown} ${OS_VERSION:-unknown}. Supported: Debian 12/13, Ubuntu 22.04/24.04." ;;
  esac
}

check_artifacts() {
  [ -x "$BIN_SRC" ] || die "missing executable binary: ${BIN_SRC}"
  [ -r "${GUI_SRC}/index.html" ] || die "missing GUI build: ${GUI_SRC}/index.html"
}

check_dpkg_state() {
  if command -v fuser >/dev/null 2>&1; then
    if fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock >/dev/null 2>&1; then
      die "apt/dpkg lock is active. Wait for package manager to finish, then rerun installer."
    fi
  fi
  if dpkg --audit | grep -q .; then
    dpkg --audit >&2 || true
    die "dpkg reports interrupted package state. Suggested manual recovery: dpkg --audit; apt-get -f install"
  fi
}

install_packages() {
  info "Installing base runtime packages"
  apt-get update
  base_packages=(
    ca-certificates
    curl
    gnupg
    jq
    iproute2
    nftables
    iptables
    dnsmasq
    dnsutils
    unzip
    psmisc
    software-properties-common
    python3-launchpadlib
    dkms
    build-essential
    systemd
    procps
  )
  case "$OS_ID" in
    debian) base_packages+=(linux-headers-amd64) ;;
    ubuntu) base_packages+=(linux-headers-generic) ;;
  esac
  apt-get install -y "${base_packages[@]}"
}

detect_network() {
  UPLINK_IF="${GRANGER_UPLINK_IF:-$(ip -4 route show default | awk '{print $5; exit}')}"
  UPLINK_GW="${GRANGER_UPLINK_GW:-$(ip -4 route show default | awk '{print $3; exit}')}"
  PUBLIC_IP="${GRANGER_PUBLIC_IP:-$(ip -4 route get 1.1.1.1 | awk '/src/ {for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')}"
  UPLINK_IF="${UPLINK_IF:-auto}"
  UPLINK_GW="${UPLINK_GW:-auto}"
  PUBLIC_IP="${PUBLIC_IP:-auto}"
}

install_files() {
  info "Installing Granger artifacts"
  install -d -m 0755 /opt/granger /var/lib/granger /var/log/granger /usr/local/sbin
  install -d -m 0700 /etc/granger /etc/granger/upstreams /etc/granger/outputs /etc/granger/secrets
  install -m 0755 "$BIN_SRC" "$BIN_DST"
  rm -rf "$GUI_DST"
  install -d -m 0755 "$GUI_DST"
  cp -a "${GUI_SRC}/." "$GUI_DST/"
}

write_config() {
  info "Writing minimal config"
  if [ -e "$CONFIG_PATH" ]; then
    printf 'config exists, leaving it untouched: %s\n' "$CONFIG_PATH"
    return
  fi
  cat >"$CONFIG_PATH" <<EOF
server:
  public_ip: "${PUBLIC_IP}"
  uplink_if: "${UPLINK_IF}"
  uplink_gw: "${UPLINK_GW}"
  dns_interface: "lo"
  dns_listen: "127.0.0.1"
  dns_upstreams:
    - "8.8.8.8"
    - "9.9.9.9"
users: {}
outputs: {}
upstreams: {}
rules: []
EOF
  chmod 0600 "$CONFIG_PATH"
}

write_service() {
  info "Writing systemd service"
  cat >"$SERVICE_PATH" <<EOF
[Unit]
Description=Granger private routing control plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${BIN_DST} serve-gui ${LISTEN} ${GUI_DST}
Restart=on-failure
RestartSec=2s
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now granger.service
}

main() {
  need_root
  detect_os
  check_artifacts
  check_dpkg_state
  detect_network
  printf 'Granger installer\n'
  printf '  OS:      %s %s\n' "$OS_ID" "$OS_VERSION"
  printf '  Arch:    %s\n' "$ARCH"
  printf '  Uplink:  %s via %s\n' "$UPLINK_IF" "$UPLINK_GW"
  printf '  Source:  %s\n' "$PUBLIC_IP"
  printf '  Listen:  http://%s\n' "$LISTEN"
  install_packages
  install_files
  write_config
  write_service
  printf '\nGranger installed.\n'
  printf 'Open: http://%s\n' "$LISTEN"
  printf 'Protocol packages are installed later from the GUI.\n'
}

main "$@"
