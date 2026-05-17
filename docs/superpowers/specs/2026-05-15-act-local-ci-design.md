# Local CI with act — Design Spec

**Date:** 2026-05-15
**Status:** Approved

## Problem

The project currently maintains two CI systems in parallel:
- GitHub Actions (`.github/workflows/`) — production CI
- Drone CI (`.drone.yml`) — self-hosted CI

This creates double maintenance burden: every CI change must be applied twice, and the `ci/scripts/lib.sh` shell abstraction layer exists solely to paper over differences between the two platforms.

## Goal

Consolidate to a single CI definition (GitHub Actions format) that:
1. Runs on GitHub Actions (production)
2. Can be validated locally before pushing, using `act`
3. Eliminates `.drone.yml` and all Drone-specific code paths

## Approach

Use **nektos/act** — a CLI tool that runs GitHub Actions workflows locally by reading `.github/workflows/` directly. Configured with `-P ubuntu-latest=-self-hosted` to execute jobs directly on the host machine (no Docker-in-Docker), giving access to the local Docker daemon and k3d.

## Architecture

### What is removed
- `.drone.yml` — deleted entirely
- `lib.sh` Drone branches — `drone)` cases in all `detect_*` functions
- `docker_login_ghcr()` Drone credential variables (`ghcr_username`, `ghcr_password`)

### What is added
- `.actrc` — act configuration (committed to git)
- `.secrets.act` — local secrets file (gitignored)
- `.github/workflows/unit-test.yml` — GHA currently has no standalone unit-test workflow

### What changes
- `lint.yml` — replaced `run-lint.sh` call with native `golangci-lint-action` (handles binary caching internally)
- `unit-test.yml` — new workflow using `actions/setup-go` with module cache
- `lib.sh` — Drone branches removed, ~140 lines deleted

### What stays the same
- `e2e.yml` — continues calling `run-e2e-tests.sh`
- `run-e2e-tests.sh`, `run-release.sh` — no changes
- `release.yml`, `docs.yml`, `pages.yml` — no changes

## Workflow File Design

### lint.yml (updated)
```yaml
name: lint
on: [pull_request, push]
jobs:
  golangci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.64.5
          args: --timeout 5m
```

### unit-test.yml (new)
```yaml
name: Unit Test
on: [pull_request, push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: bash ci/scripts/run-unit-tests.sh
```

## act Configuration

### .actrc (committed)
```
-P ubuntu-latest=-self-hosted
--secret-file .secrets.act
```

### .secrets.act (gitignored)
```
GITHUB_TOKEN=<personal_access_token>
ghcr_username=<github_username>
ghcr_password=<personal_access_token>
IS_PRO=false
```

### Local usage
```bash
act push                                          # all push-triggered workflows
act push -W .github/workflows/lint.yml            # lint only
act push -W .github/workflows/unit-test.yml       # unit tests only
act push -W .github/workflows/e2e.yml             # e2e (needs Docker + k3d on host)
```

## lib.sh Changes

All `detect_*` functions drop their `drone)` branch. Result:

```bash
detect_ci() {
  [ -n "${GITHUB_ACTIONS:-}" ] && echo "github" || echo "local"
}
```

`docker_login_ghcr()` removes `ghcr_username`/`ghcr_password` fallback — only `GITHUB_TOKEN`/`GITHUB_ACTOR` remain (these are always set by GitHub Actions; locally they come from `.secrets.act`).

## Known Limitations

| Limitation | Impact |
|---|---|
| `actions/cache` does not work in `-self-hosted` host mode | e2e does not use it; Go module cache works via host `GOPATH` |
| GitHub-specific contexts (`github.event.pull_request.labels`) require an event JSON payload locally | Use `act pull_request -e event.json` for label-based triggers |
| Image push steps are skipped locally (no `IS_PRO=true` secret by default) | Intentional — local runs only validate, not publish |

## Migration Steps

1. Simplify `lib.sh` — remove Drone branches
2. Update `lint.yml` — replace `run-lint.sh` with `golangci-lint-action`
3. Add `unit-test.yml`
4. Add `.actrc` and `.secrets.act` to `.gitignore`
5. Install act locally: `brew install act`
6. Validate: `act push -W .github/workflows/lint.yml`
7. Delete `.drone.yml`
8. Remove Drone-specific Makefile entries if any
