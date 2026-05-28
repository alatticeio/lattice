# Archived Sandbox Specs

These design specs were superseded by `docs/superpowers/specs/2026-05-28-sentry-process-sandbox-design.md`.

The new design unifies all sandbox functionality into a single approach:
- gVisor Sentry as a Go library (process sandboxing)
- Embedded gVisor netstack with WireGuard (network isolation)
- Single binary, zero external dependencies

## Superseded specs

| File | What it covered |
|------|----------------|
| `2026-05-12-home-redesign-agent-sandbox.md` | Early sandbox redesign exploration |
| `2026-05-16-agent-sandbox-security-review-and-observability.md` | Security review of old runsc-based sandbox |
| `2026-05-18-sandbox-agent-architecture.md` | Sandbox agent architecture (pre-unified design) |
| `2026-05-27-sandbox-demo-design.md` | Try Sandbox demo modal design |
| `2026-05-27-unified-agent-sandbox-design.md` | Unified `agent run/sidecar/init` design |
