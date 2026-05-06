# CI/CD Workflow Analysis

## Workflow Overview

The project has **8 GitHub Actions workflows**:

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `lint.yml` | PR, push | golangci-lint |
| `build-and-deploy.yml` | PR with label, push to master | Unit tests → build/push images → deploy to self-hosted K3s |
| `e2e.yml` | Push to dev, PR with `run-e2e` label | Build images → setup k3d → deploy → run E2E tests |
| `release.yml` | Tag `v*`, PR with `ok-release` label | GoReleaser binaries + Docker image (latticed only) |
| `build-k3s-image.yml` | Changes to k3s/deploy paths | Build + smoke test lattice-k3s image |
| `test-script.yml` | PR against master/main | Validate README CLI commands end-to-end |
| `docs.yml` | Push to master (docs path) | Build VitePress docs → deploy to alatticeio.github.io |
| `pages.yml` | Release published, manual | Build RPM/APT repo indexes on gh-pages branch |

---

## Release Pipeline (`release.yml`)

### What it does

1. **GoReleaser** — Builds binaries for `lattice` (linux/darwin/windows, amd64/arm64) and `latticed` (linux, amd64/arm64), creates archives, RPM/DEB packages, Homebrew formula, and publishes a GitHub Release with changelog.
2. **Docker** — Builds and pushes `latticed` Docker image (community edition) to GHCR with multi-arch support (linux/amd64, linux/arm64).

### What's missing for a complete release

| Gap | Impact | Status |
|-----|--------|--------|
| **No Pro binaries** in GoReleaser | Pro customers can't download Pro binaries from GitHub Releases | ✅ Fixed — `lattice-pro` + `latticed-pro` builds with `-tags pro` added |
| **No Pro Docker images** | Pro customers can't deploy `latticed:latest-pro` | ✅ Fixed — matrix build in `release.yml` builds Pro + community for both services |
| **No `lattice` agent Docker image** | Agent Docker image not published on release | ✅ Fixed — `lattice` community + Pro images added to matrix |
| **No `manager` Docker image** | K8s operator image not published on release | ❌ Not needed — manager is K8s-only, deployed via kustomize |
| **No Helm chart publishing** | Helm users can't install via `helm install` | ❌ Open — not yet scoped |
| **Package repo not updated** | `pages.yml` runs independently, not chained from release | ❌ Open — requires workflow chaining |

---

## CI/CD Pipeline (`build-and-deploy.yml`)

Builds and pushes `latest` (on master push) or `pr-N` tags for `latticed` and `lattice` images. Then deploys PR images to a self-hosted K3s server.

**Note:** Only community edition — no Pro build in this pipeline.

---

## E2E Pipeline (`e2e.yml`)

The most comprehensive pipeline. Supports:
- **Community** builds (default)
- **Pro** builds (when `run-pro` label is set on PR or `[run-pro]` in dev commit message)
- Builds both `latticed` and `lattice` images for each edition
- Deploys to k3d, runs full E2E test suite
- Collects diagnostic logs on failure

---

## Other Workflows

| Workflow | Notes |
|----------|-------|
| `lint.yml` | Runs on every PR/push — no issues |
| `build-k3s-image.yml` | Builds all-in-one K3s image with smoke tests |
| `test-script.yml` | Validates README CLI commands against live containers |
| `docs.yml` | Deploys VitePress docs site |
| `pages.yml` | Builds APT/RPM repo metadata on `gh-pages` branch |

---

## Release Readiness Checklist

Before cutting a release, verify these are in place:

- [ ] `.goreleaser.yaml` includes Pro builds (`-tags pro`) for `lattice` and `latticed`
- [ ] `release.yml` builds + pushes Pro Docker images (`EDITION=pro`)
- [ ] `release.yml` builds + pushes `lattice` agent Docker image (not just `latticed`)
- [ ] Helm chart is published (manual `helm package` + gh-pages, or automated in CI)
- [ ] Package repo index is refreshed (make `pages.yml` triggerable from release)
- [ ] Pro license public key is embedded or injectable in Pro builds (see `internal/license/keys_pro.go`)
