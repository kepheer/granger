# Changelog

## 1.0.0-rc.1 — 2026-10-30

First release candidate of the Granger VPN gateway and web control panel.

### Added

- Managed WireGuard, AmneziaWG, OpenVPN, Xray and sing-box entrypoints with individual users, downloadable profiles, QR codes and revocation.
- Personal upstream allowlists and default routes across supported entrypoints. A forbidden shared route falls back to the user's allowed default; traffic is blocked when no permitted default exists.
- Version-protected routing graph, dry-run previews and atomic access updates.
- Tunnel restoration after reboot and runtime reconciliation when interfaces change.
- Administrative token sessions, accessible dialogs, responsive navigation and consistent shadcn-svelte form controls.
- Unbounded SVG favicon.
- GitHub-built Linux amd64 archives, standalone installers and SHA256 checksums. Pre-release tags are published as GitHub prereleases.

### Fixed

- Driver packages are installed explicitly from the interface, rather than during base installation.
- Debian13 AmneziaWG prerequisites and current Xray installation options.
- Empty runtime rows, installation logs, unavailable/empty states, focus handling and revoked profile actions.
- Uninstallation stops Granger-owned tunnels and services, removes owned configuration symlinks and IPv6 firewall hooks, and preserves an administrator WireGuard file when requested.
- Disabled SNX upstreams can be restored before the optional SNX package is installed, without hiding real service-stop failures.
- Downloaded release archives are checked against SHA256SUMS before extraction.

### Validation and limitations

- WireGuard, AmneziaWG, OpenVPN and Xray were tested with real client traffic after reboot on Debian13/amd64.
- Corporate SNX-RS authentication needs a gateway and credentials; UI authentication steps were exercised with fixtures.
- Native sing-box end-to-end remains unverified due to repository availability during validation.
- Personal routing is IPv4; managed proxy IPv6 is blocked. Domain rules use DNS/ipset, not HTTP hostname or SNI inspection.
- Manual UI/accessibility checks are not full WCAG AA certification.
