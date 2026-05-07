# Project Structure Restructuring Design

**Date:** 2026-04-29
**Status:** Draft — pending implementation plan

## Problem Statement

The current project structure has several issues that hinder maintainability:

1. **`internal/infra` is a dump package** — 35 Go files, 3267 lines covering unrelated concerns (tunneling, provisioning, signaling, networking, firewall)
2. **Duplicate WRRP** — `wrrper/` (root) and `pkg/wrrp/` maintain separate implementations
3. **`management/` is a monolith** — entire server control plane in one top-level directory mixing DTOs, VOs, services, controllers, repositories
4. **Floating top-level packages** — `node/`, `turn/`, `dns/`, `wrrper/` sit at root instead of under `internal/`
5. **Unclear `pkg/` boundary** — `pkg/cmd/` duplicates `cmd/` structure; should be internal
6. **`config\` typo directory** — backslash in directory name

## Target Structure

```
lattice/
├── cmd/
│   ├── lattice/          # Agent CLI (up, status, workspace, token, policy)
│   ├── latticed/         # All-in-one daemon
│   ├── manager/          # Control plane (server + controller + turn + wrrp)
│   └── wrrper/           # Standalone relay server
├── internal/
│   ├── agent/            # Agent/node runtime (was root-level node/)
│   │   ├── wireguard/    # WG interface management (wg.go, wg_windows.go, status.go, node.go)
│   │   ├── heartbeat/    # Heartbeat logic
│   │   ├── provision/    # OS-specific provisioning (linux/darwin/windows)
│   │   ├── tunnel/       # NAT traversal: ICE, STUN, signaling, peer, endpoint, flow
│   │   ├── client/       # CLI client for server communication (was pkg/cmd/)
│   │   ├── infra/        # Low-level: conn, transport, firewall, net, device_conf, tun
│   │   ├── ipam/         # (moved from internal/ipam)
│   │   ├── controller/   # (moved from internal/controller)
│   │   ├── nats/         # (moved from internal/nats)
│   │   ├── store/        # (moved from internal/store)
│   │   ├── config/       # (moved from internal/config)
│   │   ├── log/          # (moved from internal/log)
│   │   └── wferrors/     # (moved from internal/wferrors)
│   ├── server/           # Management plane (was root-level management/)
│   │   ├── api/          # HTTP handlers, middleware, Gin setup
│   │   ├── service/
│   │   ├── controller/
│   │   ├── model/
│   │   ├── dto/
│   │   ├── vo/
│   │   ├── repository/
│   │   ├── transport/    # NATS, WS
│   │   ├── dex/
│   │   └── nats/
│   ├── relay/            # Combined WRRP + TURN (deduplicated)
│   ├── dns/              # DNS resolver (was root-level dns/)
│   ├── telemetry/        # (moved from internal/telemetry)
│   ├── proto/            # (moved from internal/proto)
│   ├── grpc/             # (moved from internal/grpc)
│   └── web/              # (moved from internal/web - frontend dist)
├── pkg/
│   ├── version/          # Shared version info
│   └── utils/            # Shared utilities (jwt, strings, format, hash, etc.)
├── api/v1alpha1/         # CRD types (unchanged)
├── fronted/              # Web dashboard (unchanged)
├── deploy/               # (unchanged)
├── config/               # K8s manifests (unchanged)
├── docs/                 # (unchanged)
├── hack/                 # (unchanged)
└── test/                 # (unchanged)
```

## Migration Strategy

### Phase 1: Move root-level packages into internal/

| From | To | Notes |
|------|-----|-------|
| `node/` | `internal/agent/` | Agent runtime |
| `turn/` | `internal/relay/` | Merged with wrrper |
| `dns/` | `internal/dns/` | Simple move |
| `wrrper/` | `internal/relay/` | Merged with turn |
| `management/` | `internal/server/` | Server control plane |
| `pkg/cmd/` | `internal/agent/client/` | CLI client library |
| `config\` | (delete) | Typo directory, remove |

### Phase 2: Break up internal/infra

`internal/infra` (35 files) splits into:

| Target Package | Files |
|---------------|-------|
| `internal/agent/tunnel/` | `ice.go`, `signal.go`, `signal_posix.go`, `peer.go`, `peer_test.go`, `endpoint.go`, `flow.go`, `message.go`, `drp.go`, `sticky.go`, `mux_filter.go`, `controlfns.go`, `command.go`, `domain.go`, `state.go`, `context.go`, `wrrp.go` |
| `internal/agent/provision/` | `provisioner.go`, `provision_linux.go`, `provision_darwin.go`, `provision_windows.go` |
| `internal/agent/infra/` | `conn.go`, `chan_conn.go`, `client.go`, `transport.go`, `net.go`, `net_test.go`, `dialer.go`, `firewall_test.go`, `device_conf.go`, `tun.go`, `tun_darwin.go`, `tun_linux.go`, `tun_windows.go`, `mark_default.go` |

### Phase 3: Deduplicate WRRP

- Merge `pkg/wrrp/` (protocol: pool.go, protocol.go, stream.go) + `wrrper/` (client/server: client.go, server.go, client_quic.go, server_quic.go, conn.go) → `internal/relay/`
- Keep the protocol definitions from `pkg/wrrp/` and the client/server from `wrrper/`
- Delete the redundant package

### Phase 4: Clean up pkg/

- `pkg/cmd/` → moved to `internal/agent/client/` (not truly public)
- `pkg/utils/` → stays (shared across binaries)
- `pkg/version/` → stays (shared across binaries)

### Phase 5: Update all imports

Update every `import` path across the codebase to reflect new locations.

## Import Path Changes

| Old Prefix | New Prefix |
|-----------|-----------|
| `github.com/alatticeio/lattice/node` | `github.com/alatticeio/lattice/internal/agent` |
| `github.com/alatticeio/lattice/turn` | `github.com/alatticeio/lattice/internal/relay` |
| `github.com/alatticeio/lattice/wrrper` | `github.com/alatticeio/lattice/internal/relay` |
| `github.com/alatticeio/lattice/management/...` | `github.com/alatticeio/lattice/internal/server/...` |
| `github.com/alatticeio/lattice/pkg/cmd` | `github.com/alatticeio/lattice/internal/agent/client` |
| `github.com/alatticeio/lattice/internal/infra` (tunnel files) | `github.com/alatticeio/lattice/internal/agent/tunnel` |
| `github.com/alatticeio/lattice/internal/infra` (provision files) | `github.com/alatticeio/lattice/internal/agent/provision` |
| `github.com/alatticeio/lattice/internal/infra` (remaining) | `github.com/alatticeio/lattice/internal/agent/infra` |
| `github.com/alatticeio/lattice/internal/ipam` | `github.com/alatticeio/lattice/internal/agent/ipam` |
| `github.com/alatticeio/lattice/internal/controller` | `github.com/alatticeio/lattice/internal/agent/controller` |
| `github.com/alatticeio/lattice/internal/nats` | `github.com/alatticeio/lattice/internal/agent/nats` |
| `github.com/alatticeio/lattice/internal/store` | `github.com/alatticeio/lattice/internal/agent/store` |
| `github.com/alatticeio/lattice/internal/config` | `github.com/alatticeio/lattice/internal/agent/config` |
| `github.com/alatticeio/lattice/internal/log` | `github.com/alatticeio/lattice/internal/agent/log` |
| `github.com/alatticeio/lattice/internal/wferrors` | `github.com/alatticeio/lattice/internal/agent/wferrors` |
| `github.com/alatticeio/lattice/internal/telemetry` | `github.com/alatticeio/lattice/internal/telemetry` |
| `github.com/alatticeio/lattice/internal/proto` | `github.com/alatticeio/lattice/internal/proto` |
| `github.com/alatticeio/lattice/internal/grpc` | `github.com/alatticeio/lattice/internal/grpc` |
| `github.com/alatticeio/lattice/internal/web` | `github.com/alatticeio/lattice/internal/web` |
| `github.com/alatticeio/lattice/dns` | `github.com/alatticeio/lattice/internal/dns` |

## Build Verification

After all moves and import updates, `make build-all` must pass.
