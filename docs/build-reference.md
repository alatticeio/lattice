# Build & Release Reference

## Makefile Commands

### Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE` | (required) | Target service: `manager`, `lattice` (agent), `latticed` (all-in-one), `lrp` (relay) |
| `EDITION` | `community` | Build edition: `community` (no build tags) or `pro` (adds `-tags pro`) |
| `TAG` | `dev` | Docker image tag |
| `VERSION` | `dev` | Version string injected via ldflags |
| `REGISTRY` | `ghcr.io/alatticeio` | Container registry |
| `TARGETOS` | `linux` | Target OS for Go build |
| `TARGETARCH` | `amd64` | Target arch for Go build |
| `ENV` | `dev` | Deployment environment (for kustomize overlays) |
| `BUILD_CACHE_ARGS` | (empty) | Docker BuildKit cache flags (set `--cache-from type=gha --cache-to type=gha,mode=max` in CI) |

### Build Commands

| Command | Description | Example |
|---------|-------------|---------|
| `make build` | Build a single service binary. For `manager`/`latticed`, builds UI first. Outputs to `bin/<SERVICE>`. | `make build SERVICE=latticed` — community binary<br>`make build SERVICE=latticed EDITION=pro` — Pro binary |
| `make build-all` | Build all services (`manager`, `lattice`, `latticed`, `lrp`) as community binaries. | `make build-all` |
| `make build-ui` | Build the Vue 3 frontend. Runs `pnpm install && pnpm build` in `fronted/`, outputs to `internal/web/dist/`. | `make build-ui` |
| `make ebpf-gen` | Generate eBPF Go bindings from `tc_ingress.bpf.c`. On macOS, uses Homebrew LLVM at `/opt/homebrew/opt/llvm/bin/clang`. | `make ebpf-gen` |

### Test Commands

| Command | Description | Example |
|---------|-------------|---------|
| `make test` | Run all unit tests (except e2e) with envtest. Requires `setup-envtest` which downloads K8s API binaries for test mocking. | `make test` |
| `make test-latticed` | Run latticed-specific tests: `cmd/latticed`, `internal/nats`, `internal/db`. | `make test-latticed` |
| `make e2e-setup` | One-click E2E environment setup: creates k3d cluster, builds Docker images for `manager` + `lattice`, imports into k3d, deploys via kustomize. | `make e2e-setup` |
| `make test-e2e` | Run E2E tests with port-forwarding. Requires a running k3d cluster with Lattice deployed. | `make test-e2e` |
| `make e2e` | Combined: `e2e-setup` + `test-e2e`. Cluster persists after tests for inspection. | `make e2e` |
| `make e2e-teardown` | Destroy the E2E k3d cluster and remove its kubeconfig. | `make e2e-teardown` |

### Docker Commands

| Command | Description | Example |
|---------|-------------|---------|
| `make docker-build` | Build a single Docker image. For `manager`/`latticed`, builds UI first. | `make docker-build SERVICE=lattice`<br>`make docker-build SERVICE=latticed EDITION=pro` |
| `make docker-build-all` | Build Docker images for all 4 services. | `make docker-build-all` |
| `make docker-push` | Push a single Docker image to `REGISTRY`. | `make docker-push SERVICE=latticed TAG=v0.1.0` |
| `make docker-push-all` | Push all service images. | `make docker-push-all` |
| `make docker-all` | Build + push all images. | `make docker-all` |
| `make docker` | Build + push single image. | `make docker SERVICE=latticed` |
| `make docker-buildx` | Multi-arch build with `docker buildx` for cross-platform support (linux/arm64, amd64, s390x, ppc64le). | `make docker-buildx IMG=ghcr.io/alatticeio/manager:v0.1.0` |
| `make docker-installer` | Build installer image locally (single platform). | `make docker-installer` |
| `make docker-installer-push` | Build + push multi-arch installer image via buildx (linux/amd64, arm64). | `make docker-installer-push` |

### K8s / CRD Commands

| Command | Description |
|---------|-------------|
| `make manifests` | Generate CRD YAML, ClusterRole, and WebhookConfiguration via controller-gen. Output to `config/crd/bases/`. |
| `make generate` | Generate DeepCopy methods for CRD types + compile protobuf definitions. |
| `make install` | Install CRDs into a K8s cluster. |
| `make uninstall` | Remove CRDs from a K8s cluster. |
| `make deploy` | Deploy according to `ENV` (uses kustomize overlay at `config/lattice/overlays/<ENV>/`). |
| `make undeploy` | Remove the deployment. |
| `make deploy-aio` | Deploy all-in-one mode (latticed) to K8s. Creates namespace, installs CRDs, applies kustomize manifests with image substitution. |
| `make undeploy-aio` | Remove all-in-one deployment. |
| `make build-installer` | Build kustomize output manifests into `deploy/quickstart/` for installer to fetch. |
| `make yaml` | Build all-in-one kustomize output to `config/lattice.yaml`. |

### Code Quality Commands

| Command | Description |
|---------|-------------|
| `make lint` | Run golangci-lint. |
| `make lint-fix` | Run golangci-lint with `--fix` for auto-fixable issues. |
| `make lint-config` | Verify golangci-lint configuration. |
| `make fmt` | Run `go fmt ./...`. |
| `make vet` | Run `go vet ./...`. |

### Tool Installation Commands

These auto-download the required tools to `./bin/` if not already present:

| Command | Tool | Version Source |
|---------|------|----------------|
| `make kustomize` | kustomize v5.6.0 | Hardcoded |
| `make controller-gen` | controller-tools v0.18.0 | Hardcoded |
| `make setup-envtest` | setup-envtest (matching controller-runtime) | From go.mod |
| `make golangci-lint` | golangci-lint v1.64.5 | Hardcoded |

### Build Details

**Version injection** — The following values are injected via Go ldflags:

```
-x 'lattice/pkg/version.Version=<git-describe>'
-x 'lattice/pkg/version.GitCommit=<git-rev-parse>'
-x 'lattice/pkg/version.BuildTime=<UTC timestamp>'
-x 'lattice/pkg/version.GoVersion=<go version>'
```

**UI build** — `manager` and `latticed` services embed the Vue 3 frontend via `//go:embed internal/web/dist/`. The `build-ui` target must run before building these services. Both `make build` and `make docker-build` handle this automatically.

**Community vs Pro** — The `EDITION` variable controls Go build tags:
- `community` (default): no build tags — community stubs compile in, Pro code is excluded
- `pro`: adds `-tags pro` — Pro implementations compile in, community stubs are excluded

Pro features include: TURN server, Dex OIDC/SSO, telemetry/VictoriaMetrics push, eBPF policy enforcement, dashboard analytics, network monitoring, Ed25519 JWT license verification.
