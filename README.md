# Granger

[English](README.md) | [Русский](README_ru-RU.md)

**Granger** is a self-hosted VPN routing orchestrator for Debian/Ubuntu.

Granger runs a private control plane that users connect to, while traffic is routed through different upstream tunnels based on rules: domains, CIDR ranges, default route, and fallback.

## Supported systems

* Debian 12 / 13 amd64
* Ubuntu 22.04 / 24.04 amd64

## Drivers

### Upstreams

* `direct`
* `interface`
* `wireguard`
* `openvpn`
* `amneziawg`
* `snx-rs`
* `xray`
* `sing-box`

### Entrypoints / outputs

* `wireguard`
* `openvpn`
* `amneziawg`
* `xray`
* `sing-box`

## Quick installation via curl

Latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/kepheer/granger/main/scripts/install-granger.sh | sh
```

Specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/kepheer/granger/main/scripts/install-granger.sh | GRANGER_VERSION=v0.1.0 sh
```