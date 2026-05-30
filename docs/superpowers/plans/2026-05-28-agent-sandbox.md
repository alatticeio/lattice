# AI Agent Sandbox Implementation Plan

> **Goal:** Implement community edition: kernel wg0 in container netns, fork AI agent, ~80 lines.

## Tasks

### Task 1: Revert run_community.go to kernel wg0

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/run_community.go`
- Delete: `cmd/lattice/cmd/sandbox/shared_linux.go`
- Delete: `cmd/lattice/cmd/sandbox/run_community_stub.go`

Revert to kernel wg0 approach. Restore helpers (registerOrResume, runPeriodicRefresh, forkAndWait, parseEgressCIDRs) into the file. Update help text. Build tag: `//go:build !pro`.

### Task 2: Rewrite run_pro.go to kernel wg0

Revert to kernel wg0 approach with egress flags. Build tag: `//go:build pro && linux`.

### Task 3: Update go.mod

Remove lattice-shim sentry dependency.

### Task 4: Update frontend

Command: `docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice sandbox run ...`

### Task 5: Verify build, lint, test
