# Lattice CLI Design

> Status: Approved | 2026-05-01

## Motivation

The original CLI communicated with the control plane via NATS for all operations — including CRUD management commands (`token create`, `workspace add`, `policy add`). This created several problems:

1. **NATS URL coupling**: Every CLI command needed to know the NATS URL, even for non-real-time operations
2. **No standard auth**: NATS-based auth lacked the simplicity of Bearer JWT tokens
3. **Config sprawl**: Multiple config structs, scattered flag definitions, inconsistent parameter names
4. **Redundant parameters**: Users had to explicitly configure `signaling-url`, `server-url`, `relay-url`, and more, even though several could be auto-discovered

The redesign consolidated around HTTP REST for management, NATS only for real-time signaling, and a unified config system with auto-discovery.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  lattice CLI                         │
│  init │ login │ up │ status │ token │ workspace │...│
└──────────────────┬──────────────────────────────────┘
                   │
          ┌────────▼────────┐
          │  ConfigManager  │  6-layer onion loading
          │  (Viper)        │
          └────────┬────────┘
                   │
    ┌──────────────┼──────────────┐
    │              │              │
    ▼              ▼              ▼
┌────────┐  ┌────────────┐  ┌──────────────┐
│ ~/.lat │  │ LATTICE_*  │  │ CLI flags    │
│ tice/  │  │ env vars   │  │ (--key=val)  │
│ *.yaml │  │            │  │              │
└────────┘  └────────────┘  └──────────────┘

Management commands (HTTP REST):
  token, workspace, policy, peer
  → Bearer JWT auth (lattice login)
  → X-Workspace-Id header
  → /api/v1/* endpoints

Real-time signaling (NATS):
  peer discovery, ICE candidates, config push
  → auto-discovered from /api/v1/discovery
```

---

## 1. Command Hierarchy

```
lattice
├── init              Interactive config setup
├── login             Authenticate, save JWT to config
├── up                Start agent (connect as peer)
├── status            Show WireGuard interface status
├── version           Show client + server version
├── token
│   ├── create        Create enrollment token
│   ├── list          List tokens
│   └── remove        Revoke token
├── workspace
│   ├── add           Create workspace
│   ├── remove        Delete workspace
│   └── list          List workspaces
├── policy
│   ├── add           Create policy
│   ├── allow-all     Default allow-all shortcut
│   ├── remove        Delete policy
│   └── list          List policies
└── peer
    ├── list          List peers
    └── label         Set peer labels
```

### Persistent Flags (root level, shared by all subcommands)

| Flag | Default | Description |
|------|---------|-------------|
| `--config-dir` | `~/.lattice` | Config directory |
| `--server-url` | `""` | Management server URL |
| `--save` | `false` | Persist CLI flags to config file |

### `up` Command Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--token` | `""` | Enrollment token |
| `--level` | `info` | Log level |
| `--relay-url` | `:6266` | TCP relay URL |
| `--relay-quic-url` | `""` | QUIC relay URL |
| `--enable-lrp` | `false` | Enable LRP relay transport |
| `--vm-endpoint` | `""` | VictoriaMetrics push endpoint |
| `--enable-metric` | `false` | Expose Prometheus metrics |
| `--enable-sys-log` | `false` | Verbose WG/ICE debug logging |
| `--wg-port` | `51820` | WireGuard/ICE UDP port |
| `--enable-daemon` / `-d` | `false` | Fork as background daemon |

### Key Design Decisions

- **No `lattice down` command**: Agent stop is handled by `agent.Stop()` (SIGTERM via PID file). Not exposed as a CLI command.
- **No non-interactive `init`**: Init is always interactive; use `--save` on other commands for non-interactive setup.
- **Management via HTTP REST**: token, workspace, policy, peer commands all use REST API, not NATS.

---

## 2. `lattice init` — Interactive Setup

### Flow

```
1. Resolve config path: ~/.lattice/lattice.yaml
2. If exists → prompt "Overwrite? [y/N]"
3. Prompt for required:
   - Management server URL (--server-url)
   - Enrollment token (--token)
4. Prompt for optional (Enter to skip):
   - Relay TCP URL (--relay-url)
   - Relay QUIC URL (--relay-quic-url)
5. Write to Viper → Save to lattice.yaml
6. Print next steps: lattice login → lattice up
```

### What gets created

```yaml
# ~/.lattice/lattice.yaml
server-url: http://<host>:8080
token: <enrollment-token>
relay-url: :6266              # optional
relay-quic-url: host:6267     # optional
```

---

## 3. Config System — 6-Layer Onion Loading

### Priority (lowest → highest)

```
1. Hard-coded defaults
2. lattice.yaml (base config file)
3. lattice.{env}.yaml (environment override)
4. LATTICE_* environment variables
5. CLI flags (--key=value)
6. K8s service discovery fallback (for server-url)
```

### Unified Config Struct

A single `Config` struct serves both agent and server:

```go
type Config struct {
    ServerUrl     string  // Management API URL
    Token         string  // Enrollment token
    AuthToken     string  // JWT from lattice login
    SignalingURL  string  // NATS URL (auto-discovered)
    RelayURL      string  // TCP relay address
    RelayQuicURL  string  // QUIC relay address
    AppId         string  // Node identity (auto-generated)
    WgPort        int     // WireGuard/ICE UDP port, default 51820
    Level         string  // Log level, default "info"
    EnableLrp     bool
    EnableDaemon  bool
    // ... + nested blocks for Database, AI, Dex, JWT, etc.
}
```

### Default Values

| Field | Default | Source |
|-------|---------|--------|
| `listen` | `:8080` | hardcoded |
| `level` | `info` | hardcoded |
| `env` | `dev` | hardcoded |
| `stun-url` | `stun.alattice.io:3478` | hardcoded |
| `relay-url` | `:6266` | hardcoded |
| `wg-port` | `51820` | hardcoded |
| `database.driver` | `sqlite` | hardcoded |
| `server-url` | `""` | must configure |
| `signaling-url` | `""` | auto-discovered |

### `--save` Flag

When `--save` is passed on any command (e.g., `lattice up --token X --server-url Y --save`), only explicitly-changed flags are written back to `lattice.yaml`. This allows one-shot config persistence without a separate `init` step.

### K8s Fallbacks

If `ServerUrl` is empty after all layers, the system tries K8s service discovery env vars:
- `LATTICE_MANAGER_SERVICE_HOST`
- `LATTICE_API_SERVICE_HOST`
- `MANAGER_SERVICE_HOST`

---

## 4. NATS URL Auto-Discovery

### Problem

Users shouldn't need to know or configure NATS URLs. The NATS server is embedded in `latticed` (all-in-one) or deployed internally in the cluster. Requiring `--signaling-url` is unnecessary friction.

### Protocol

```
Agent                           Control Plane
  │                                  │
  │  GET /api/v1/discovery           │
  ├─────────────────────────────────>│
  │                                  │  Read SignalingURL from:
  │                                  │  1. System config DB
  │                                  │  2. Server config
  │                                  │  3. Fallback: nats://127.0.0.1:4222
  │  {"data": {"nats_url": "..."}}   │
  │<─────────────────────────────────┤
```

**Agent-side logic:**

```go
if config.GetSignalingURL() == "" {
    natsURL := discoverNATSURL(serverURL + "/api/v1/discovery")
    config.SetSignalingURL(natsURL)  // stored in runtime field, not persisted
}
```

**Key design details:**
- Discovery only runs if `SignalingURL` is empty (user can override via `LATTICE_SIGNALING_URL` or `--signaling-url`)
- The discovered URL is stored in an unexported `runtimeNATSURL` field — it's runtime-only, not persisted to `lattice.yaml`
- `GetSignalingURL()` checks `runtimeNATSURL` first, then falls back to the config file value

---

## 5. The HTTP REST Redesign

### What Changed

**Commit `4778ac69`** — replaced NATS-based CLI management with HTTP REST.

| Aspect | Before | After |
|--------|--------|-------|
| Transport | NATS pub/sub | HTTP REST |
| Auth | NATS subject-based | JWT Bearer token |
| Workspace context | Embedded in NATS payload | `X-Workspace-Id` header |
| Client | `nats.Conn` publish | HTTP client with JSON envelope |
| Admin handler file | `nats_admin.go` (427 lines) | Removed; REST endpoints in `api.go` |

### Management API Client

All CLI management commands use a shared HTTP client:

```go
type Client struct {
    BaseURL string           // server-url
    token   string           // JWT from lattice login
}
```

Every request:
1. `Authorization: Bearer <token>`
2. `X-Workspace-Id: <workspace-id>` (resolved from namespace)
3. Responses unwrapped from JSON envelope: `{ code, msg, data }`

### `lattice login`

```
POST /api/v1/users/login
Body: { username, password, client: "cli" }
Response: { user, token }
```

- CLI login gets **30-day JWT** (vs 12-hour for web)
- JWT persisted to `lattice.yaml` as `auth-token`
- Used by all subsequent management commands

---

## 6. Agent Startup — `lattice up`

### Flow

```
1. Validate config (server-url + token required)
2. Generate AppId if not set (hostname-derived)
3. [Optional] Daemon fork (LATTICE_DAEMON=true)
4. Discover NATS URL from /api/v1/discovery
5. Three-phase initialization:
   Phase 1: TUN device, UDP sockets, NATS client
   Phase 2: Register with server, get identity + IP + relay URL
   Phase 3: WireGuard device, DefaultBind, Provisioner
6. Start: Up(), GetNetworkMap(), ApplyFullConfig()
```

### Phase 1: Network Foundation
- Create `wf0` TUN device
- Open UDP sockets (v4 + v6)
- Create FilteringUDPMux (shared WireGuard + ICE)
- Connect NATS signal service

### Phase 2: Identity & Signaling
- Register with control plane via NATS
- Receive: private key, overlay IP, relay URL, peer list
- Build KeyManager, ProbeFactory
- Subscribe NATS signal topics
- [Optional] Start LRP relay client

### Phase 3: WireGuard Data Plane
- DefaultBind (with ICE + LRP paths)
- WireGuard Device on `wf0`
- Provisioner (iptables or eBPF)
- MessageHandler (config push from control plane)

### Background Tasks
- Heartbeat goroutine
- VictoriaMetrics push (optional)
- UAPI IPC listener (for `wg set` compatibility)
- WireGuard handshake watcher (optional)

---

## 7. File Layout

```
cmd/lattice/main.go                        CLI entry point
cmd/lattice/cmd/root.go                    Root command, persistent flags
cmd/lattice/cmd/init.go                    Interactive config setup
cmd/lattice/cmd/login.go                   JWT authentication
cmd/lattice/cmd/up.go                      Agent startup
cmd/lattice/cmd/status.go                  WireGuard status display
cmd/lattice/cmd/version.go                 Version info
cmd/lattice/cmd/token/token.go             Enrollment token management
cmd/lattice/cmd/workspace/workspace.go     Workspace management
cmd/lattice/cmd/policy/policy.go           Network policy management
cmd/lattice/cmd/peer/peer.go               Peer inspection/labeling
internal/agent/config/config.go            Config struct, manager, onion loading
internal/agent/config/config_test.go       Config loading tests
internal/agent/node.go                     Node struct, NATS discovery, 3-phase init
internal/agent/run.go                      Agent start/stop, daemon fork
internal/agent/client/client.go            HTTP client for management commands
internal/agent/client/admin.go             Workspace/policy/token/peer REST methods
internal/server/client/client.go           NATS-based register/getNetmap client
internal/server/server/api.go              HTTP API router, discovery handler
```
