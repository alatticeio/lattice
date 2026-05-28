# Legacy Sandbox Archive

Archived on 2026-05-27. These files represent the old `lattice sandbox` command
implementation before the unified `lattice agent` redesign.

## What was replaced

| Directory | Description |
|-----------|-------------|
| `cmd-sandbox/` | `cmd/lattice/cmd/sandbox/` — old sandbox command tree (start/run/agent, pod mode, gVisor/runsc mode) |
| `internal-runsc/` | `internal/agent/runsc/` — RunscDriver two-phase OCI bundle manager (Phase 1: kernel wg0, Phase 2: runsc container) |
| `e2e/` | `test/e2e/agent_sandbox_gvisor_test.go` — E2E test for the now-removed gVisor runsc mode |

## Why replaced

See design spec: `docs/superpowers/specs/2026-05-27-unified-agent-sandbox-design.md`

Key problems with the old design:
- `RunscDriver` created a **kernel wg0** in Phase 1 (contradicts the "no kernel wg0" goal)
- Three divergent code paths: community sandbox, pod mode (PRO), gVisor/runsc mode (PRO)
- SOCKS5 proxy required `ALL_PROXY` awareness in AI agents
- Mac/non-Linux users could not run any sandbox

## New design

- `lattice agent run` — single container, `--runtime=runsc`, no kernel wg0
- `lattice agent sidecar` — K8s sidecar, gVisor netstack embedded + transparent proxy
- `lattice agent init` — K8s init container, iptables REDIRECT rules
