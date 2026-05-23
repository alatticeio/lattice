# Communication

**语言要求：始终用中文回答用户的所有问题和消息。**

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
cmd/          # Entry points: lattice, latticed, manager, lrp, lrper
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
make ebpf-gen                 # Generate eBPF bindings (requires LLVM with BPF target)
make EDITION=pro build        # PRO build with -tags pro
```

### PRO/Community Build Tag Pattern

```go
//go:build pro     → PRO edition (compiled with -tags pro)
//go:build !pro    → Community stub (default, no build tag)
```

Makefile: `EDITION ?= community` → default no tags. `EDITION=pro` adds `-tags pro`.

Community stubs return `402 Payment Required` or `errors.New("... is a Pro feature")`.

### eBPF Build Environment

macOS requires Homebrew LLVM for BPF cross-compilation (Apple Xcode clang doesn't support BPF target):

```bash
brew install llvm
```

The Makefile sets `BPF2GO_CC=/opt/homebrew/opt/llvm/bin/clang` on macOS to pick the correct compiler.

**Important:** Do NOT add `-cc clang` to the `//go:generate` directive in `doc.go` — it overrides the `BPF2GO_CC` env var from the Makefile and causes "No available targets are compatible with triple 'bpfel'" on macOS.

## Git Workflow

- **Branches**: `master` (main), `dev` (development)
- **Commits**: Conventional commits with scope: `feat(scope):`, `fix(scope):`, `refactor:`, `ci:`
- **Commit grouping**: 同一功能点（相同 prefix，如 `feat(agent-isolation):`）的所有修改必须合并到**同一个 commit** 中，不要分散成多次提交。创建新 commit 前，先用 `git log --oneline -20` 检查最近提交，若已有相同 prefix 的 commit，使用 `git commit --amend` 追加到该 commit 而非新建。
- **Commit flags**: Always use `git commit -s` (DCO sign-off)
- **Commit author**: Always use the identity from `git config user.name` / `git config user.email` — never add `Co-Authored-By` trailers, 提交之前先跑一下'make lint'检查有没有lint errors,如果有直接提示并修复
- **PR triggers**: `run-docker`, `run-e2e`, `run-helm`, `run-readme`, `run-benchmark` labels (see `.github/PR_LABELS.md`)

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

ICE (direct P2P) races with LRP (relay fallback). State machine manages lifecycle:
`Created → Probing → ICEReady/LRPReady → Failed → Closed`

## Frontend

```bash
cd fronted && pnpm install && pnpm dev    # Dev server
cd fronted && pnpm build                  # Build → internal/web/dist/
```

UI is embedded in Go binary via `//go:embed dist/` in `internal/web/`.
