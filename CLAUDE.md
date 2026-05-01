# Lattice Project Context

> Kubernetes-native overlay networking with WireGuard. Open-core model: community + PRO editions.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25.0 |
| HTTP API | Gin |
| Database | GORM + SQLite (default) / MySQL |
| Signaling | NATS |
| Networking | WireGuard + ICE (pion) + QUIC |
| K8s | controller-runtime, kubebuilder CRDs |
| Frontend | Vue 3.5 + Vite + pnpm + Tailwind 4 |

## Directory Structure

```
cmd/          # Entry points: lattice, latticed, manager, lrp, wrrper
internal/     # Private: agent, server, relay, db, grpc, etc.
api/v1alpha1/ # CRD types: LatticeNetwork, LatticePeer, LatticePolicy
config/       # kustomize: crd, rbac, lattice (all-in-one, dev overlays)
fronted/      # Vue 3 frontend (note: directory name is misspelled)
test/e2e/     # Ginkgo e2e tests
pkg/          # Shared: utils, version
```

## Build System

```bash
make build                    # Build default service
make build SERVICE=manager    # Build specific service
make build-ui                 # Build Vue frontend → internal/web/dist/
make lint                     # golangci-lint
make test                     # Unit tests
make test-e2e                 # E2E tests (needs k3d cluster)
make ebpf-gen                 # Generate eBPF bindings
make EDITION=pro build        # PRO build with -tags pro
```

### PRO/Community Build Tag Pattern

```go
//go:build pro     → PRO edition (compiled with -tags pro)
//go:build !pro    → Community stub (default, no build tag)
```

Makefile: `EDITION ?= community` → default no tags. `EDITION=pro` adds `-tags pro`.

Community stubs return `402 Payment Required` or `errors.New("... is a Pro feature")`.

## Git Workflow

- **Branches**: `master` (main), `dev` (development)
- **Commits**: Conventional commits with scope: `feat(scope):`, `fix(scope):`, `refactor:`, `ci:`
- **PR triggers**: `run-e2e` label, `run-pro` label, `[run-pro]` in commit message, `ok-release` label

## Code Patterns

### Logging
```go
import "github.com/alatticeio/lattice/internal/agent/log"

logger := log.GetLogger("module-name")
logger.Info("message", "key", value)
logger.Error("message", err, "key", value)  // err as second arg
logger.Warn("message", "key", value)
logger.Debug("message", "key", value)
```

### Error handling
- Wrap with `fmt.Errorf("context: %w", err)`
- Sentinel errors with `errors.New("...")`
- Return early on errors, don't nest

### File headers
All Go files start with Apache 2.0 license boilerplate.

### Naming
- Service structs: lowercase (`userService`), interfaces: CamelCase (`UserService`)
- Factory functions: `NewXxx()` returns interface

## Testing

- Framework: Ginkgo v2 + Gomega
- K8s tests: `envtest` with real CRDs from `config/crd/bases/`
- E2E: `test/e2e/` — requires `make e2e-setup` (k3d cluster)
- Unit: co-located `*_test.go` or under package
- Suite bootstrap: `suite_test.go` per controller

## Linting

- golangci-lint v1.64.5
- Enabled: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `asciicheck`, `bodyclose`
- `_test.go` files skip `errcheck` and `unused`
- Run: `make lint` or `bin/golangci-lint run ./...`

## Key Architecture

| Component | Entry | Purpose |
|-----------|-------|---------|
| Lattice Agent | `cmd/lattice` | Edge node, WireGuard tunnel, NATS signaling |
| LatticeD | `cmd/latticed` | All-in-one control plane (NATS + SQLite + API + UI) |
| Manager | `cmd/manager` | K8s operator, reconciles CRDs |

### Policy Enforcement (iptables/eBPF)

`PolicyEnforcer` interface in `internal/agent/provision/provisioner.go` abstracts the backend:
- Community: iptables (default)
- PRO + Linux 5.10+: eBPF TC on wf0 TUN interface
- `SelectEnforcerMode()` decides at startup; falls back to iptables if eBPF unavailable
- BPF source: `internal/agent/ebpf/tc_ingress.bpf.c`

### Transport

ICE (direct P2P) races with WRRP (relay fallback). State machine manages lifecycle:
`Created → Probing → ICEReady/WRRPReady → Failed → Closed`

## Frontend

```bash
cd fronted && pnpm install && pnpm dev    # Dev server
cd fronted && pnpm build                  # Build → internal/web/dist/
```

UI is embedded in Go binary via `//go:embed dist/` in `internal/web/`.
