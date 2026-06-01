# Granger

[English](README.md) | [Русский](README_ru-RU.md)

**Granger** — self-hosted VPN routing orchestrator для Debian/Ubuntu.

Granger поднимает приватный control plane, через который пользователи подключаются к серверу, а трафик маршрутизируется через разные upstream-туннели по правилам: домены, CIDR, default route, fallback.

## Поддерживаемые системы

* Debian 12 / 13 amd64
* Ubuntu 22.04 / 24.04 amd64

## Драйверы

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

## Быстрая установка через curl

Последний release:

```sh
curl -fsSL https://raw.githubusercontent.com/kepheer/granger/main/scripts/install-granger.sh | sh
```

Конкретная версия:

```sh
curl -fsSL https://raw.githubusercontent.com/kepheer/granger/main/scripts/install-granger.sh | GRANGER_VERSION=v0.1.0 sh
```
