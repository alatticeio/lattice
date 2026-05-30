# Agent Binary Size Optimization Design

**Date:** 2026-05-28  
**Status:** Approved  
**Goal:** Reduce `lattice` agent binary size from ~37MB (Linux production) to ~20MB

---

## Background

After merging PRO and community editions into a single binary, the `lattice` agent binary grew because server-side dependencies leaked into the agent through shared packages. The agent only needs WireGuard, ICE/QUIC transport, NATS client, eBPF, and gVisor (for sandbox subcommands) — but currently also pulls in gin, gorm, sqlite, MySQL driver, redis, and k8s.io/apimachinery.

**Current sizes (Linux, production `-s -w`):**

| Binary | Size |
|--------|------|
| `lattice` (agent + CLI + sandbox) | ~37MB |
| `latticed` (all-in-one server) | ~80MB |
| `manager` (K8s operator) | ~75MB |

The agent binary is the focus.

---

## Root Cause Analysis

Dependency chain that introduces server-side bloat into the agent:

```
cmd/lattice
  → internal/agent
      → internal/server/client       (→ server/dto)
      → internal/server/nats
      → internal/server/transport
  → internal/agent/client            (→ server/vo → gorm + k8s.io/apimachinery)
  → internal/agent/store             (→ server/models → gorm)
  → pkg/utils                        (→ gin/binding + redis/go-redis + server/models)
  → api/v1alpha1                     (→ k8s.io/apimachinery)
```

**Heavy packages that should not be in the agent:**

| Package | Symbols in agent | Root cause |
|---------|-----------------|------------|
| `gin` + `ugorji/go` codec | 5500+ | `pkg/utils/validator.go` imports `gin/binding` |
| `gorm` + sqlite + mysql | ~5MB | `server/vo/label.go` has `gorm.DeletedAt`; `pkg/utils/jwt.go` imports `server/models` |
| `redis/go-redis` | ~3MB | `pkg/utils/redis.go` |
| `k8s.io/apimachinery` | 2471 | `server/vo` imports `api/v1alpha1`; `agent/client` imports `server/vo` |

---

## Design

### Change 1: Split `pkg/utils` — move server-only helpers out

**Problem:** `pkg/utils` mixes pure utilities with server-only helpers.

**Files to move** to `internal/server/utils/`:
- `pkg/utils/redis.go` → `internal/server/utils/redis.go`
- `pkg/utils/validator.go` (gin binding) → `internal/server/utils/validator.go`
- `pkg/utils/jwt.go` (imports `server/models`) → `internal/server/utils/jwt.go`

All server-side callers of these functions update their import paths. `pkg/utils` retains only pure, dependency-free utilities.

**Removes from agent:** `gin`, `ugorji/go` codec, `redis/go-redis`, `server/models` → gorm chain.

---

### Change 2: Remove gorm from `server/vo`

**Problem:** `internal/server/vo/label.go` uses `gorm.DeletedAt`, making the entire `server/vo` package depend on gorm — and agent pulls in `server/vo` via `agent/client`.

**Fix:** Replace `gorm.DeletedAt` with `*time.Time` in the VO struct. Soft-delete timestamp is display-only in VOs; gorm's `DeletedAt` type is only needed in model layer (`server/models`).

**Removes from agent:** `gorm.io/gorm` and transitively sqlite + MySQL driver.

---

### Change 3: Replace `agent/client` → `server/vo` dependency with local view types

**Problem:** `internal/agent/client` uses `server/vo` types for CLI output (`lattice peer list`, etc.). Since `server/vo` now imports `api/v1alpha1`, this pulls in `k8s.io/apimachinery`.

**Fix:** Define lightweight local structs in `internal/agent/client` for CLI display. These contain only the JSON fields needed for terminal output, with no embedded gorm or k8s types. The HTTP response from the server is decoded directly into these local structs (they match the JSON shape).

**Removes from agent:** `k8s.io/apimachinery` and `api/v1alpha1` K8s CRD types.

---

### Change 4: Isolate `agent/store` from `server/models`

**Problem:** `internal/agent/store` defines the `Store` interface with method signatures using `*models.User`, `*models.Workspace`, etc. — gorm-annotated model types.

**Fix:** Store interface methods use `dto.*` types (already exist, are pure structs) or simple primitive parameters where appropriate. The concrete implementation in `db/gormstore` continues to use `*models.*` internally — this is already behind the interface boundary. Only the interface definition needs to be decoupled.

**Removes from agent:** remaining `server/models` → gorm import path.

---

### Change 5: Add `-trimpath` to Makefile build target

**Problem:** Build path strings from the compilation host are embedded in the binary.

**Fix:** Add `-trimpath` flag to the `go build` invocation in the `build` Makefile target.

```makefile
# Before
-ldflags="-s -w $(LDFLAGS)"

# After
-trimpath -ldflags="-s -w $(LDFLAGS)"
```

This removes absolute filesystem paths from the binary (~1MB saving) and is also a security best practice (no build machine path leakage).

---

## Expected Outcome

| Change | Heavy dependency removed | Estimated saving |
|--------|--------------------------|-----------------|
| Split `pkg/utils` | gin + ugorji/go codec + redis | ~8MB |
| Remove gorm from `server/vo` | gorm + sqlite + MySQL driver | ~5MB |
| Local view types in `agent/client` | k8s.io/apimachinery | ~3MB |
| Isolate `agent/store` | remaining gorm path | ~1MB |
| Add `-trimpath` | build path strings | ~1MB |
| **Total** | | **~18MB, ~37MB → ~19MB** |

---

## Non-Goals

- Splitting `lattice` into multiple binaries (single binary requirement preserved)
- Adding `nosandbox` build tag to exclude gVisor (not in scope for this iteration)
- Optimizing `latticed` or `manager` binary size (separate effort)
- UPX compression (increases startup latency, not suitable for production daemons)

---

## Testing

- `go list -deps ./cmd/lattice/ | grep -E "gorm|redis|gin|k8s.io/apimachinery"` must return empty after changes
- All existing unit tests pass (`make test`)
- Lint passes (`make lint`)
- Manual smoke test: `lattice up`, `lattice peer list`, `lattice sandbox run` (Linux) still work
