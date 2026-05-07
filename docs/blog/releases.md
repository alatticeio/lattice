# Release Notes

## v0.2.0 (2026-05)

### Highlights

- **AI Agent**: MCP server integration, intent engine (Pro), time-travel debugging (Pro), compliance automation (Pro)
- **Signaling Redesign**: HTTP REST replaces NATS for CLI management
- **Policy Engine**: Refactored rule engine with eBPF support (Pro) and comprehensive test coverage
- **Network Peering**: Cluster and network peering with status cards
- **Audit & Telemetry**: Audit logging, member management revamp, i18n support

### Breaking Changes

- CLI management moved from NATS to HTTP REST. Update any automation scripts.
- Policy CRD structure changed. Re-apply policies after upgrade.

### Upgrade

```bash
curl -fsSL https://get.lattice.io | sh -s -- --version 0.2.0
lattice upgrade
```

## v0.1.3

First tagged release. WireGuard mesh with ICE NAT traversal, LRP relay, and basic dashboard.
