# PR Labels & CI Triggers

## Workflow Trigger Reference

| Workflow | File | PR (auto) | PR (label) | push `dev` | push `master` | push `v*` tag | schedule | Notes |
|----------|------|:---------:|:----------:|:----------:|:-------------:|:-------------:|:--------:|-------|
| **lint** | `lint.yml` | ✅ all PRs | — | ✅ | ✅ | — | — | golangci-lint |
| **unit-test** | `unit-test.yml` | ✅ all PRs | — | ✅ | ✅ | — | — | `go test ./internal/... ./pkg/...` |
| **goreleaser** | `release.yml` | ✅ all PRs (snapshot) | — | — | ✅ (snapshot) | ✅ (release) | — | PR: snapshot only; tag: full binary release |
| **docker** | `release.yml` | — | `run-docker` (build only, no push) | — | ✅ (push `latest`) | ✅ (push `v*` + `latest`) | — | |
| **docs build** | `docs.yml` | ✅ `docs/**` changed | — | — | ✅ + deploy | — | — | PR: build only; master: deploy to GitHub Pages |
| **e2e** | `e2e.yml` | — | `run-e2e` | ✅ | — | — | — | k3d cluster, ~30min |
| **helm-deploy** | `helm-deploy.yml` | — | `run-helm` | ✅ | — | — | — | Helm install + smoke test, ~5min |
| **test-readme** | `test-readme-script.yml` | — | `run-readme` | — | — | — | — | Also auto-runs after Release workflow succeeds on master; ~40min |
| **benchmark** | `benchmark.yml` | — | `run-benchmark` (no store) | ✅ (store) | ✅ (store) | — | Mon 02:00 UTC (store) | Results only stored on push/schedule |
| **build-k3s-image** | `build-k3s-image.yml` | ✅ `deploy/k3s/**` or Go src changed | — | ✅ same paths | ✅ + push image | — | — | Builds community + pro `lattice-k3s` images |
| **pages** | `pages.yml` | — | — | — | — | — | — | Disabled (placeholder only) |

## PR Labels

Add these labels to a PR to trigger on-demand workflows:

| Label | Workflow | Description |
|-------|----------|-------------|
| `run-docker` | `release.yml` → docker job | Build Docker images (no push), validates Dockerfile and image build |
| `run-e2e` | `e2e.yml` | Full E2E tests on k3d cluster (~30min) |
| `run-helm` | `helm-deploy.yml` | Helm chart deploy to k3d cluster with smoke test (~5min) |
| `run-readme` | `test-readme-script.yml` | Validates all CLI commands documented in README (~40min) |
| `run-benchmark` | `benchmark.yml` | Run component and integration benchmarks (results not stored) |

## Manual trigger (workflow_dispatch)

All workflows support manual triggering from the GitHub Actions page, useful for debugging or one-off runs.

## Post-merge triggers

| Event | Action |
|-------|--------|
| push to master | goreleaser snapshot, Docker build + push `latest`, deploy docs |
| push `v*` tag | Full release: goreleaser binaries + Docker `v*` + `latest` + Helm chart OCI publish |
