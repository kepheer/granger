#!/bin/sh
set -eu

APP_NAME="granger"
REPO="${GRANGER_REPO:-kepheer/granger}"
VERSION="${GRANGER_VERSION:-latest}"
LISTEN="${GRANGER_LISTEN:-10.19.84.51:1984}"
WG_INTERFACE="${GRANGER_ADMIN_WG_INTERFACE:-wg0}"
WG_SUBNET="${GRANGER_ADMIN_WG_SUBNET:-10.19.84.0/24}"
WG_SERVER_IP="${GRANGER_ADMIN_WG_SERVER_IP:-10.19.84.51}"
WG_CLIENT_IP="${GRANGER_ADMIN_WG_CLIENT_IP:-10.19.84.52}"
WG_PORT="${GRANGER_ADMIN_WG_PORT:-51820}"
BIN_DST="/usr/local/sbin/granger"
GUI_DST="/opt/granger/gui"
CONFIG_PATH="/etc/granger/granger.yaml"
SERVICE_PATH="/etc/systemd/system/granger.service"
TMP_DIR=""
BIN_SRC="${GRANGER_BIN:-}"
GUI_SRC="${GRANGER_GUI_SRC:-}"

export DEBIAN_FRONTEND="${DEBIAN_FRONTEND:-noninteractive}"
export NEEDRESTART_MODE="${NEEDRESTART_MODE:-a}"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

info() {
  printf '\n==> %s\n' "$*"
}

cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

need_root() {
  [ "$(id -u)" -eq 0 ] || die "run as root: curl -fsSL URL | sh"
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

  headers="linux-headers-amd64"
  kernel="linux-image-amd64"
  if [ "$OS_ID" = "ubuntu" ]; then
    headers="linux-headers-generic"
    kernel="linux-image-generic"
  fi

  apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gnupg \
    jq \
    iproute2 \
    nftables \
    iptables \
    ipset \
    dnsmasq \
    dnsutils \
    unzip \
    psmisc \
    software-properties-common \
    python3-launchpadlib \
    dkms \
    build-essential \
    systemd \
    procps \
    wireguard-tools \
    "$kernel" \
    "$headers"
}

check_running_kernel_headers() {
  if [ ! -e "/lib/modules/$(uname -r)/build" ]; then
    printf '\nreboot required: headers for the running kernel %s are unavailable.\n' "$(uname -r)" >&2
    printf 'Reboot the server and rerun the Granger installer.\n' >&2
    exit 75
  fi
}

script_dir() {
  case "${0:-}" in
    /*|*/*)
      dir="$(dirname -- "$0")"
      (cd -- "$dir" 2>/dev/null && pwd) || true
      ;;
    *) true ;;
  esac
}

detect_local_artifacts() {
  if [ -n "$BIN_SRC" ] && [ -n "$GUI_SRC" ]; then
    return 0
  fi

  dir="$(script_dir)"
  if [ -n "$dir" ]; then
    for base in "$dir" "$dir/.."; do
      if [ -x "$base/granger" ] && [ -r "$base/dist/gui/index.html" ]; then
        BIN_SRC="$base/granger"
        GUI_SRC="$base/dist/gui"
        return 0
      fi
    done
  fi

  return 1
}

latest_tarball_url() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*linux_amd64\.tar\.gz\)".*/\1/p' |
    head -n 1
}

version_tarball_url() {
  version="$1"
  clean="${version#v}"
  printf 'https://github.com/%s/releases/download/%s/granger_%s_linux_amd64.tar.gz\n' "$REPO" "$version" "$clean"
}

download_artifacts() {
  if detect_local_artifacts; then
    info "Using local artifacts"
    return
  fi

  TMP_DIR="$(mktemp -d)"
  url="${GRANGER_TARBALL_URL:-}"
  if [ -z "$url" ]; then
    if [ "$VERSION" = "latest" ]; then
      info "Resolving latest release"
      url="$(latest_tarball_url)"
    else
      url="$(version_tarball_url "$VERSION")"
    fi
  fi
  [ -n "$url" ] || die "could not resolve Granger release tarball URL"

  info "Downloading ${url}"
  curl -fsSL "$url" -o "$TMP_DIR/granger.tar.gz"
  tar -xzf "$TMP_DIR/granger.tar.gz" -C "$TMP_DIR"

  BIN_SRC="$(find "$TMP_DIR" -type f -name granger -perm -111 | head -n 1)"
  GUI_SRC="$(find "$TMP_DIR" -type f -path '*/dist/gui/index.html' | head -n 1)"
  GUI_SRC="$(dirname "$GUI_SRC")"

  [ -x "$BIN_SRC" ] || die "downloaded release does not contain executable granger binary"
  [ -r "$GUI_SRC/index.html" ] || die "downloaded release does not contain dist/gui"
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
  cp -a "$GUI_SRC/." "$GUI_DST/"
}

write_admin_wireguard() {
  info "Writing private admin WireGuard entrypoint"
  if [ -e "$CONFIG_PATH" ]; then
    printf 'config exists, leaving admin WireGuard entrypoint untouched: %s\n' "$CONFIG_PATH"
    return
  fi
  install -d -m 0700 /etc/wireguard /etc/granger/outputs /etc/granger/secrets

  server_private="$(wg genkey)"
  server_public="$(printf '%s\n' "$server_private" | wg pubkey)"
  client_private="$(wg genkey)"
  client_public="$(printf '%s\n' "$client_private" | wg pubkey)"

  cat >"/etc/granger/secrets/admin_wg-server.key" <<EOF
${server_private}
EOF
  chmod 0600 "/etc/granger/secrets/admin_wg-server.key"

  cat >"/etc/granger/outputs/${WG_INTERFACE}.conf" <<EOF
[Interface]
PrivateKey = ${server_private}
Address = ${WG_SERVER_IP}/24
ListenPort = ${WG_PORT}

[Peer]
PublicKey = ${client_public}
AllowedIPs = ${WG_CLIENT_IP}/32
EOF
  chmod 0600 "/etc/granger/outputs/${WG_INTERFACE}.conf"
  ln -sfn "/etc/granger/outputs/${WG_INTERFACE}.conf" "/etc/wireguard/${WG_INTERFACE}.conf"

  cat >"/root/granger-admin.conf" <<EOF
[Interface]
PrivateKey = ${client_private}
Address = ${WG_CLIENT_IP}/32
DNS = ${WG_SERVER_IP}

[Peer]
PublicKey = ${server_public}
Endpoint = ${PUBLIC_IP}:${WG_PORT}
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
EOF
  chmod 0600 /root/granger-admin.conf
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
  dns_interface: "${WG_INTERFACE}"
  dns_listen: "${WG_SERVER_IP}"
  dns_upstreams:
    - "8.8.8.8"
    - "9.9.9.9"
users:
  admin:
    display_name: "Administrator"
    output: "admin_wg"
    client: "admin"
outputs:
  admin_wg:
    type: "wireguard"
    interface: "${WG_INTERFACE}"
    config: "/etc/granger/outputs/${WG_INTERFACE}.conf"
    subnet: "${WG_SUBNET}"
    server_ip: "${WG_SERVER_IP}"
    listen_port: ${WG_PORT}
    clients:
      - name: "admin"
        user: "admin"
        ip: "${WG_CLIENT_IP}/32"
        config: "/root/granger-admin.conf"
        public_key: "${client_public}"
upstreams:
  direct:
    type: "direct"
    interface: "${UPLINK_IF}"
    gateway: "${UPLINK_GW}"
    default: true
rules:
  - name: "default"
    default: true
    via: "direct"
EOF
  chmod 0600 "$CONFIG_PATH"
}

write_service() {
  info "Writing systemd service"
  cat >"$SERVICE_PATH" <<EOF
[Unit]
Description=Granger private routing control plane
After=network-online.target wg-quick@${WG_INTERFACE}.service
Wants=network-online.target
Requires=wg-quick@${WG_INTERFACE}.service

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
  check_dpkg_state
  download_artifacts
  detect_network

  printf 'Granger installer\n'
  printf '  OS:      %s %s\n' "$OS_ID" "$OS_VERSION"
  printf '  Arch:    %s\n' "$ARCH"
  printf '  Uplink:  %s via %s\n' "$UPLINK_IF" "$UPLINK_GW"
  printf '  Source:  %s\n' "$PUBLIC_IP"
  printf '  Listen:  http://%s\n' "$LISTEN"

  install_packages
  check_running_kernel_headers
  install_files
  write_admin_wireguard
  write_config
  if [ -r "/etc/wireguard/${WG_INTERFACE}.conf" ]; then
    systemctl enable --now "wg-quick@${WG_INTERFACE}.service"
  else
    die "WireGuard entrypoint config is missing: /etc/wireguard/${WG_INTERFACE}.conf"
  fi
  "${BIN_DST}" apply
  write_service

  printf '\nGranger installed.\n'
  printf 'Open: http://%s\n' "$LISTEN"
  if [ -r /root/granger-admin.conf ]; then
    printf 'Admin WireGuard config: /root/granger-admin.conf\n'
    printf '\n'
    cat /root/granger-admin.conf
    printf '\n'
  fi
  printf 'Protocol packages are installed later from the GUI.\n'
}

main "$@"
