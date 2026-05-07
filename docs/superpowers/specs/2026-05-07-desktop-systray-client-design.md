# Desktop System Tray Client Design

> Status: Approved | 2026-05-07

## Motivation

Lattice is an enterprise-oriented WireGuard overlay network product that also serves individual users. Currently, end users interact with Lattice through a CLI (`lattice init/up/status`) or a Web dashboard. However:

- Enterprise users (developers, data analysts, product teams, external contractors) are often not comfortable with CLI tools
- There is no always-visible connection status indicator — users cannot tell at a glance whether they are connected
- All competitors (Tailscale, Netbird, ZeroTier, Twingate) ship desktop clients — this is table stakes for enterprise procurement

A desktop system tray client closes these gaps with minimal development effort.

## Goal

Deliver a lightweight cross-platform desktop system tray application that provides:

1. Visual connection status (tray icon: grey/green/red)
2. Right-click menu: Connect/Disconnect, Network Details, Open Web Dashboard, Quit
3. Pop-up status panel showing peer count, overlay IP, TTFH, connection scenario
4. Auto-start on boot
5. Cross-platform: macOS menubar, Windows systray, Linux appindicator

## Non-Goals (MVP)

- Full settings UI (use Web dashboard)
- Traffic graphs / charts
- Multi-workspace switching
- Dark mode settings
- In-app analytics

## Architecture

```
lattice-client/
├── go/                     # Wails Shell (Go)
│   ├── main.go             # Wails app entry
│   ├── systray.go          # Tray icon + menu
│   ├── wgstatus.go         # WireGuard status via wgctrl
│   └── api.go              # API calls to control plane
├── frontend/               # Vue 3 + Tailwind (reuses fronted/ components)
│   ├── index.html
│   ├── src/
│   │   ├── App.vue         # Root: empty unless panel open
│   │   ├── StatusPanel.vue # Pop-up status panel
│   │   └── main.ts
│   └── wails.json
├── wails.json              # Wails project config
└── Makefile
```

### Component Responsibilities

**Go Shell:**
- Manage system tray lifecycle (platform-specific: macOS menubar, Windows systray, Linux appindicator)
- Read local WireGuard state via `wgctrl` (reuses logic from `internal/agent/wireguard/`)
- Call control plane API for peer count, workspace name (`GET /api/v1/...`)
- Expose status data to Vue frontend via Wails bindings
- Handle Connect/Disconnect by calling `lattice up/down` equivalent
- Open Web dashboard URL in default browser

**Vue Frontend:**
- Minimal status panel: peer count, overlay IP, TTFH, connection scenario
- Reuses components from `fronted/src/components/ui/` where appropriate (button, badge, alert)
- No routing — single-page panel invoked from tray

**Build & Distribution:**
- Wails builds produce platform-native binaries with embedded WebView
- macOS: `.app` bundle, notarized
- Windows: `.exe` with installer (NSIS)
- Linux: `.deb`, `.rpm`, AppImage (reuse existing packaging pipeline)
- WebView2 is system-provided on Windows (pre-installed since Win 10 1809+), WKWebView on macOS (built-in), WebKitGTK on Linux

## Technology Choice

**Wails (Go + WebView)** was chosen over alternatives:

| Option | Binary Size | Stack Reuse | UX | Rationale |
|--------|------------|-------------|-----|-----------|
| Wails | ~12-18MB | Go + Vue 3 | Native | Best fit: Go backend reuse, Vue 3 component sharing |
| Tauri | ~5-10MB | Rust + Vue 3 | Native | Rust is not in Lattice's tech stack |
| Electron | ~50MB+ | Node + Vue 3 | Good | Too heavy; conflicts with "lightweight" positioning |
| Go + Native UI | ~3MB | Go | Platform-specific | High maintenance cost for 3 platforms |

Netbird uses the same approach (Wails), providing validation of this choice in the same product category.

## System Tray UX

### Tray Icon States

| State | Color | Meaning |
|-------|-------|---------|
| Disconnected | Grey | Agent not running or not enrolled |
| Connected | Green | WireGuard tunnel active, at least one peer reachable |
| Error | Red | Agent running but tunnel handshake failed |

### Right-Click Menu

```
[Status: Connected · 10.96.0.4]
─────────────────────────────
 ✓ Connected
   Disconnect
─────────────────────────────
   Network Details...
   Open Web Dashboard
─────────────────────────────
   Quit Lattice
```

### Status Panel (left-click pop-up)

Compact card (300x240px) showing:
- Workspace name
- Overlay IP
- Connection scenario (LAN / NAT / Relay)
- TTFH (last handshake duration)
- Peer count (online / total)
- Last connected time

## Binary Size

Target: 12-18MB uncompressed, 6-8MB compressed.
Rationale: Tailscale is ~10MB. 12-18MB is acceptable given the additional WebView + Vue runtime overhead.

## Integration with Existing Code

- **WireGuard state**: Reuses `internal/agent/wireguard/` package (same `wgctrl` calls as the agent)
- **Frontend components**: Shares `fronted/src/components/ui/button`, `badge`, `alert` via path aliases
- **API client**: Reuses `fronted/src/api/` module for control plane communication
- **Build pipeline**: New `make build-client` target; CI adds Wails build step to `build-and-deploy` workflow
- **Distribution**: Adds `lattice-client` binary to existing Brew/YUM/APT/GitHub Releases pipeline

## Dependencies & Risks

| Risk | Mitigation |
|------|-----------|
| WebView2 missing on older Windows | Detect at install, prompt user to install (one-time) |
| Linux WebKitGTK version fragmentation | AppImage bundles known-good runtime |
| macOS notarization | CI step with `gon` notary tool + Apple Developer account |
| Wails API stability | Pin Wails v2.x, test upgrades in CI before adopting |
