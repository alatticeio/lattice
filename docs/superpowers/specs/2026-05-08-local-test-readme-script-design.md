# Local Test-README Script Design

**Date:** 2026-05-08
**Status:** Approved

## Problem

The `test-readme-script.yml` CI workflow validates README CLI commands end-to-end, but can only run in GitHub Actions. Every code change requires merging to master or waiting for a PR build to see results. Developers need a way to run the same test locally without rebuilding images unnecessarily.

## Goal

Extract the CI workflow logic into `scripts/test-readme.sh` so it can be run locally with `bash scripts/test-readme.sh`, while the CI workflow calls the same script — one source of truth for both environments.

## Script: `scripts/test-readme.sh`

### Flags

| Flag | Description |
|------|-------------|
| `--force-build` | Force rebuild of both images even if local versions exist |

### Image Strategy

Both `lattice-k3s` and `lattice` agent images follow the same resolution order:

```
1. K3S_IMAGE / AGENT_IMAGE env var set? → use it (CI passes pr-N tag)
2. --force-build? → build/pull fresh
3. Local image exists (lattice-k3s:local / lattice:local)? → use it
4. Otherwise → build lattice-k3s from source / pull lattice from GHCR
```

Local builds are tagged `:local` to distinguish from registry images:
- `lattice-k3s:local` — built from `deploy/k3s/Dockerfile`
- `lattice:local` — built locally if available, otherwise `ghcr.io/alatticeio/lattice:latest`

### Cleanup Behaviour

- **Success** → automatically remove all containers and the `wf-test-net` bridge network
- **Failure** → leave containers running; print debugging instructions:

```
=== TEST FAILED ===
Containers preserved for debugging:
  docker logs lattice-k3s
  docker logs wf-agent-a
  docker logs wf-agent-b
To clean up manually:
  docker rm -f lattice-k3s wf-agent-a wf-agent-b
  docker network rm wf-test-net
```

### Script Flow

Steps map 1:1 to the existing workflow steps:

1. Parse arguments (`--force-build`)
2. Resolve images (per strategy above)
3. Start `lattice-k3s` container (`--privileged -p 8080:8080`)
4. Wait for K8s API + CRDs + latticed `:8080` ready
5. Login → obtain `AUTH_TOKEN`
6. Set `LATTICE="docker run --rm --network host -e LATTICE_AUTH_TOKEN=... <image>"`
7. Step 1: `workspace add` / `workspace list`
8. Step 2: `token create` / `token list` → extract `TOKEN`
9. Wait for operator to create workspace namespace
10. Step 3: start `wf-agent-a` and `wf-agent-b` on `wf-test-net` bridge
11. Wait for both agents to register (≥2 nodes)
12. Wait for LatticePeer IP allocation
13. Step 4: `policy allow-all` / `policy add` / `policy list`
14. Step 5: `lattice status` inside containers + ping A→B
15. Step 6: `token remove` / `policy remove` / `workspace remove`
16. Cleanup (success only)

## CI Integration

`test-readme-script.yml` is simplified to call the script directly:

```yaml
- name: "Run README test script"
  run: bash scripts/test-readme.sh
```

The `K3S_IMAGE` and `AGENT_IMAGE` env vars remain in the workflow and are read by the script, allowing CI to pass PR-specific tags (`pr-N`) without any changes to the script logic.

The existing `build-k3s-image.yml` workflow continues to build and push images on PRs and master pushes — the test script just consumes whatever image is available.

## Local Usage

```bash
# First run — builds lattice-k3s:local from source (~5 min)
bash scripts/test-readme.sh

# Subsequent runs — reuses lattice-k3s:local (~1 min startup)
bash scripts/test-readme.sh

# After changing deploy/k3s/ or cmd/latticed/ — force rebuild
bash scripts/test-readme.sh --force-build
```

## Files Changed

| File | Change |
|------|--------|
| `scripts/test-readme.sh` | New — extracted + enhanced from workflow |
| `.github/workflows/test-readme-script.yml` | Simplified to call the script |
