# Zero-Friction Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `POST /api/v1/demo/launch` creates a temp workspace + enrollment token and returns two curl commands; users run them on two devices to get a live WireGuard mesh in < 3 minutes.

**Architecture:** Thin handler that reuses `workspaceController.AddWorkspace` and `tokenController.Create`, then patches `is_demo=true/expires_at` directly on the workspace. A background goroutine in `server.go` sweeps expired demo workspaces every 5 minutes via `workspaceController.DeleteWorkspace`. The feature is off by default (`demo.enabled=false`); when on, a per-IP token-bucket rate limiter (reusing the existing `IPRateLimiter`) caps abuse. The frontend modal stores the response in `localStorage` to survive page refreshes.

**Tech Stack:** Go (Gin, GORM, controller-runtime), Vue 3 + Vite, existing `IPRateLimiter`, `workspaceController`, `tokenController`, `policyController`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/server/models/workspace.go` | Modify | Add `IsDemo bool`, `ExpiresAt *time.Time` |
| `internal/agent/store/store.go` | Modify | Add `ListExpiredDemos` to `WorkspaceRepository` interface |
| `internal/agent/config/config.go` | Modify | Add `DemoConfig` struct + embed in `AppConfig` |
| `internal/db/gormstore/workspace.go` | Modify | Implement `ListExpiredDemos` |
| `internal/server/server/demo.go` | **Create** | `LaunchDemo` handler, `DemoStatus` handler, cleanup goroutine |
| `internal/server/server/server.go` | Modify | Add `demoLimiter` field, start cleanup goroutine |
| `internal/server/server/api.go` | Modify | Register `/api/v1/demo/launch` and `/api/v1/demo/status` |
| `cmd/lattice/cmd/init.go` | Modify | Add `--server`, `--token` flags for non-interactive mode |
| `docs/public/install.sh` | Modify | Parse `--server`, `--token`, `--binary`, `--tag` CLI args |
| `frontend/src/components/DemoModal.vue` | **Create** | Demo modal with countdown, code blocks, copy buttons |
| `frontend/src/pages/index.vue` | Modify | Add "Try Demo" secondary button |

---

## Task 1: Workspace model + config

**Files:**
- Modify: `internal/server/models/workspace.go`
- Modify: `internal/agent/config/config.go`

- [ ] **Step 1: Add IsDemo and ExpiresAt to Workspace**

In `internal/server/models/workspace.go`, add after the `SeedInjected` field:

```go
// Demo workspace fields
IsDemo    bool       `gorm:"default:false" json:"isDemo"`
ExpiresAt *time.Time `gorm:"index" json:"expiresAt,omitempty"`
```

The file already imports `"time"`, so no import change needed.

- [ ] **Step 2: Add DemoConfig to AppConfig**

In `internal/agent/config/config.go`, add before the closing `}` of `AppConfig` (around line 410):

```go
Demo DemoConfig `mapstructure:"demo"`
```

Then add the new struct after `AppConfig`:

```go
// DemoConfig controls the zero-friction demo feature.
// Disabled by default; operator must set demo.enabled=true to expose the endpoint.
type DemoConfig struct {
    Enabled          bool `mapstructure:"enabled"`           // default false
    TTLMinutes       int  `mapstructure:"ttlMinutes"`        // default 60
    RateLimitPerHour int  `mapstructure:"rateLimitPerHour"` // default 5
}
```

- [ ] **Step 3: Verify GORM migration picks up new columns**

GORM AutoMigrate in `internal/db/gormstore/migrate.go` already includes `&models.Workspace{}`, so the new columns are added automatically on next startup. No change needed.

- [ ] **Step 4: Build to confirm no compile errors**

```bash
make build SERVICE=latticed
```

Expected: binary built, no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/server/models/workspace.go internal/agent/config/config.go
git commit -s -m "feat(demo): add Workspace.IsDemo/ExpiresAt and DemoConfig"
```

---

## Task 2: WorkspaceRepository — ListExpiredDemos

**Files:**
- Modify: `internal/agent/store/store.go`
- Modify: `internal/db/gormstore/workspace.go`

- [ ] **Step 1: Write failing test**

In `internal/db/gormstore/workspace_test.go`, add:

```go
var _ = Describe("workspaceRepo.ListExpiredDemos", func() {
    It("returns only expired demo workspaces", func() {
        ctx := context.Background()

        past := time.Now().Add(-1 * time.Hour)
        future := time.Now().Add(1 * time.Hour)

        expired := &models.Workspace{Slug: "demo-exp", DisplayName: "Expired", IsDemo: true, ExpiresAt: &past}
        active := &models.Workspace{Slug: "demo-act", DisplayName: "Active", IsDemo: true, ExpiresAt: &future}
        regular := &models.Workspace{Slug: "regular", DisplayName: "Regular", IsDemo: false}

        Expect(st.Workspaces().Create(ctx, expired)).To(Succeed())
        Expect(st.Workspaces().Create(ctx, active)).To(Succeed())
        Expect(st.Workspaces().Create(ctx, regular)).To(Succeed())

        results, err := st.Workspaces().ListExpiredDemos(ctx, time.Now())
        Expect(err).NotTo(HaveOccurred())
        Expect(results).To(HaveLen(1))
        Expect(results[0].ID).To(Equal(expired.ID))
    })
})
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd internal/db/gormstore && go test -run "ListExpiredDemos" -v ./...
```

Expected: compile error — `ListExpiredDemos` not defined.

- [ ] **Step 3: Add method to store interface**

In `internal/agent/store/store.go`, add to `WorkspaceRepository` interface after `ExistsByUserAndSlug`:

```go
// ListExpiredDemos returns all demo workspaces whose ExpiresAt is before cutoff.
ListExpiredDemos(ctx context.Context, cutoff time.Time) ([]*models.Workspace, error)
```

Make sure `"time"` is imported at the top of `store.go`.

- [ ] **Step 4: Implement in gormstore**

In `internal/db/gormstore/workspace.go`, add after `ExistsByUserAndSlug`:

```go
func (r *workspaceRepo) ListExpiredDemos(ctx context.Context, cutoff time.Time) ([]*models.Workspace, error) {
	var results []*models.Workspace
	err := r.DB().WithContext(ctx).
		Where("is_demo = ? AND expires_at IS NOT NULL AND expires_at < ?", true, cutoff).
		Find(&results).Error
	return results, err
}
```

Add `"time"` to imports in `workspace.go` if not already present.

- [ ] **Step 5: Run test to confirm it passes**

```bash
cd internal/db/gormstore && go test -run "ListExpiredDemos" -v ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/store/store.go internal/db/gormstore/workspace.go internal/db/gormstore/workspace_test.go
git commit -s -m "feat(demo): add WorkspaceRepository.ListExpiredDemos"
```

---

## Task 3: Demo handler

**Files:**
- Create: `internal/server/server/demo.go`

The handler must:
1. Check `cfg.App.Demo.Enabled`; return `403` if false
2. Check IP rate limit
3. Create workspace (no user context — `AddWorkspace` handles empty `userID` gracefully)
4. Patch `is_demo=true`, `expires_at=now+TTL` directly on the workspace via `store.Workspaces().Update`
5. Create enrollment token with `limit=2` and `expiry=TTLMinutes`
6. Apply default-allow policy so devices can ping each other
7. Build the curl commands from the request host
8. Return JSON response

- [ ] **Step 1: Write failing test**

Create `internal/server/server/demo_test.go`:

```go
// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDemoStatusDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.App.Demo.Enabled = false

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/demo/status", nil)

	s := &Server{cfg: cfg}
	s.handleDemoStatus()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].(map[string]interface{})
	assert.Equal(t, false, data["enabled"])
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/server/server/... -run TestDemoStatusDisabled -v
```

Expected: compile error — `handleDemoStatus` not defined.

- [ ] **Step 3: Implement demo.go**

Create `internal/server/server/demo.go`:

```go
// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// demoLaunchResponse is returned by POST /api/v1/demo/launch.
type demoLaunchResponse struct {
	WorkspaceID string    `json:"workspace_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	Device1Cmd  string    `json:"device1_cmd"`
	Device2Cmd  string    `json:"device2_cmd"`
}

func (s *Server) demoRouter() {
	s.GET("/api/v1/demo/status", s.handleDemoStatus())
	s.POST("/api/v1/demo/launch", s.demoLimiter.Middleware(rate.Limit(s.demoCfg().RateLimitPerHour)/3600, 1), s.handleDemoLaunch())
}

func (s *Server) demoCfg() *demoConfigDefaults {
	c := s.cfg.App.Demo
	d := &demoConfigDefaults{
		Enabled:          c.Enabled,
		TTLMinutes:       c.TTLMinutes,
		RateLimitPerHour: c.RateLimitPerHour,
	}
	if d.TTLMinutes <= 0 {
		d.TTLMinutes = 60
	}
	if d.RateLimitPerHour <= 0 {
		d.RateLimitPerHour = 5
	}
	return d
}

type demoConfigDefaults struct {
	Enabled          bool
	TTLMinutes       int
	RateLimitPerHour int
}

func (s *Server) handleDemoStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		resp.OK(c, gin.H{"enabled": s.cfg.App.Demo.Enabled})
	}
}

func (s *Server) handleDemoLaunch() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.demoCfg()
		if !cfg.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "demo is disabled"})
			return
		}

		ctx := c.Request.Context()
		ttl := time.Duration(cfg.TTLMinutes) * time.Minute
		expiresAt := time.Now().Add(ttl)

		// 1. Create workspace (no user context needed — AddWorkspace handles empty userID)
		slug := fmt.Sprintf("demo-%d", time.Now().UnixMilli())
		wsVo, err := s.workspaceController.AddWorkspace(ctx, &dto.WorkspaceDto{
			Slug:        slug,
			DisplayName: "Demo Workspace",
		})
		if err != nil {
			resp.Error(c, "failed to create demo workspace: "+err.Error())
			return
		}

		// 2. Mark workspace as demo with expiry
		ws, err := s.store.Workspaces().GetByID(ctx, wsVo.ID)
		if err != nil {
			resp.Error(c, "failed to fetch workspace: "+err.Error())
			return
		}
		ws.IsDemo = true
		ws.ExpiresAt = &expiresAt
		if err := s.store.Workspaces().Update(ctx, ws); err != nil {
			resp.Error(c, "failed to mark demo workspace: "+err.Error())
			return
		}

		// 3. Create enrollment token (limit=2, one per device)
		tokenCtx := context.WithValue(ctx, infra.WorkspaceKey, wsVo.ID)
		expiry := fmt.Sprintf("%dm", cfg.TTLMinutes)
		tokenStr, err := s.tokenController.Create(tokenCtx, &dto.TokenDto{
			Namespace: wsVo.Namespace,
			Limit:     2,
			Expiry:    expiry,
		})
		if err != nil {
			resp.Error(c, "failed to create enrollment token: "+err.Error())
			return
		}

		// 4. Apply allow-all policy so the two demo devices can ping each other
		if _, err := s.policyController.CreateOrUpdatePolicy(tokenCtx, wsVo.ID, wsVo.Namespace, &dto.PolicyDto{
			Name:      "demo-allow-all",
			Namespace: wsVo.Namespace,
			Action:    "Allow",
		}); err != nil {
			// Non-fatal: log and continue; demo still works but traffic is default-deny
			s.logger.Warn("failed to apply demo allow-all policy", "err", err)
		}

		// 5. Build curl commands
		scheme := "https"
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		host := c.Request.Host
		if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}
		serverURL := fmt.Sprintf("%s://%s", scheme, host)
		installURL := fmt.Sprintf("%s/install.sh", serverURL)
		cmd := fmt.Sprintf(
			"curl -fsSL %s | bash -s -- --server %s --token %s",
			installURL, serverURL, tokenStr,
		)

		resp.OK(c, demoLaunchResponse{
			WorkspaceID: wsVo.ID,
			ExpiresAt:   expiresAt,
			Device1Cmd:  cmd,
			Device2Cmd:  cmd,
		})
	}
}

// startDemoCleanup launches a background goroutine that deletes expired demo workspaces every 5 minutes.
func (s *Server) startDemoCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sweepExpiredDemos(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Server) sweepExpiredDemos(ctx context.Context) {
	expired, err := s.store.Workspaces().ListExpiredDemos(ctx, time.Now())
	if err != nil {
		s.logger.Warn("demo cleanup: list expired demos failed", "err", err)
		return
	}
	for _, ws := range expired {
		if err := s.workspaceController.DeleteWorkspace(ctx, ws.ID); err != nil {
			s.logger.Warn("demo cleanup: delete workspace failed", "id", ws.ID, "err", err)
		} else {
			s.logger.Info("demo cleanup: deleted expired demo workspace", "id", ws.ID, "namespace", ws.Namespace)
		}
	}
}
```

Note: `s.policyController.CreateOrUpdatePolicy` — check the actual method signature on `PolicyController` interface. If the interface doesn't expose this method, use the `policyController` directly or call the service. Adjust the call to match the actual interface (Task 3 step 5 below).

- [ ] **Step 4: Check PolicyController interface for the correct method signature**

```bash
grep -n "CreateOrUpdatePolicy\|interface PolicyController" internal/server/controller/policy.go
```

Look at the method names available. If `CreateOrUpdatePolicy` doesn't exist with that signature, use whatever method is available (e.g., `Apply`, `CreatePolicy`). Update the `demo.go` call to match.

- [ ] **Step 5: Run test to confirm it passes**

```bash
go test ./internal/server/server/... -run TestDemoStatusDisabled -v
```

Expected: PASS.

- [ ] **Step 6: Build to confirm no compile errors**

```bash
make build SERVICE=latticed
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/server/server/demo.go internal/server/server/demo_test.go
git commit -s -m "feat(demo): add LaunchDemo handler and cleanup goroutine"
```

---

## Task 4: Server wiring

**Files:**
- Modify: `internal/server/server/server.go`
- Modify: `internal/server/server/api.go`

- [ ] **Step 1: Add demoLimiter to Server struct**

In `internal/server/server/server.go`, add field to `Server` struct after `revocationList`:

```go
demoLimiter *middleware.IPRateLimiter
```

- [ ] **Step 2: Initialize demoLimiter in NewServer**

In `NewServer`, after the line `revocationList := auth.NewRevocationList()`, add:

```go
demoRL := middleware.NewIPRateLimiter()
```

And in the `Server{}` struct literal, add:

```go
demoLimiter: demoRL,
```

- [ ] **Step 3: Start cleanup goroutine in NewServer**

At the end of `NewServer`, just before `if err = s.apiRouter()`, add:

```go
// Demo workspace cleanup — runs even when demo is disabled (no-op when no demo workspaces exist).
s.startDemoCleanup(ctx)
```

Note: `NewServer` receives `ctx context.Context` as its first parameter — use it here.

- [ ] **Step 4: Register demo routes in apiRouter**

In `internal/server/server/api.go`, at the end of `apiRouter()` just before the `web.RegisterHandlers(s.Engine)` line, add:

```go
s.demoRouter()
```

- [ ] **Step 5: Build and verify**

```bash
make build SERVICE=latticed
```

Expected: binary built, no compile errors.

- [ ] **Step 6: Smoke test — status endpoint returns disabled**

```bash
# Start latticed with default config (demo.enabled defaults to false)
./bin/latticed &
sleep 2
curl -s http://localhost:8080/api/v1/demo/status
# Expected: {"code":0,"data":{"enabled":false}}
kill %1
```

- [ ] **Step 7: Commit**

```bash
git add internal/server/server/server.go internal/server/server/api.go
git commit -s -m "feat(demo): wire demo handler and cleanup into server"
```

---

## Task 5: lattice init non-interactive mode

**Files:**
- Modify: `cmd/lattice/cmd/init.go`

The `install.sh` script will call `lattice init --server <url> --token <tok>` after installing the binary. Currently `init` is fully interactive. We need it to accept those flags and skip prompts.

- [ ] **Step 1: Add --server and --token flags**

In `internal/agent/config/config.go`, confirm the viper keys: `server-url` and `token`. These match the existing fields in `Config`.

In `cmd/lattice/cmd/init.go`, update `initCmd()`:

```go
func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactively configure Lattice and save to config file",
		Long: `Prompt for required connection parameters and save them to
~/.lattice/lattice.yaml. After init, run "lattice up" with no flags.`,
		Example: `  lattice init
  lattice init --server https://lattice.company.com --token lt-enroll-xxx
  lattice init --config-dir /etc/lattice`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd)
		},
	}
	cmd.Flags().String("server", "", "Management server URL (non-interactive)")
	cmd.Flags().String("token", "", "Enrollment token (non-interactive)")
	return cmd
}
```

- [ ] **Step 2: Update runInit to use flags when provided**

Replace the `runInit` function body:

```go
func runInit(cmd *cobra.Command) error {
	cfgPath := config.GetConfigFilePath()
	v := cfgManager.Viper()

	// Non-interactive mode: --server and --token both provided via flags
	serverFlag, _ := cmd.Flags().GetString("server")
	tokenFlag, _ := cmd.Flags().GetString("token")
	if serverFlag != "" && tokenFlag != "" {
		v.Set("server-url", serverFlag)
		v.Set("token", tokenFlag)
		if err := cfgManager.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Config saved to %s\n", cfgPath)
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)

	// If the config file already exists, ask whether to overwrite it
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Config file already exists at %s\n", cfgPath)
		fmt.Print("Overwrite existing config? [y/N]: ")
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "y") {
			fmt.Println("Aborted. Existing config unchanged.")
			return nil
		}
	}

	// Required fields
	serverURL := prompt(scanner, "Management server URL (--server-url)", v.GetString("server-url"))
	token := prompt(scanner, "Enrollment token (--token)", v.GetString("token"))

	// Optional fields
	relayURL := promptOptional(scanner, "Relay TCP URL (--relay-url, optional)")
	relayQuicURL := promptOptional(scanner, "Relay QUIC URL (--relay-quic-url, optional)")

	v.Set("server-url", serverURL)
	v.Set("token", token)
	if relayURL != "" {
		v.Set("relay-url", relayURL)
	}
	if relayQuicURL != "" {
		v.Set("relay-quic-url", relayQuicURL)
	}

	if err := cfgManager.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nConfig saved to %s\n", cfgPath)
	fmt.Println(`Next steps:
  lattice login   — authenticate for management commands (workspace, token, policy)
  lattice up      — connect this device as a peer`)
	return nil
}
```

- [ ] **Step 3: Build and verify the flag works**

```bash
make build SERVICE=lattice
./bin/lattice init --server https://test.example.com --token lt-test-123
cat ~/.lattice/lattice.yaml
# Expected: server-url: https://test.example.com, token: lt-test-123
```

- [ ] **Step 4: Verify interactive mode still works**

```bash
echo -e "y\nhttps://manual.example.com\nlt-manual-token\n\n" | ./bin/lattice init
cat ~/.lattice/lattice.yaml
# Expected: server-url: https://manual.example.com
```

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/init.go
git commit -s -m "feat(demo): add --server/--token flags to lattice init for non-interactive mode"
```

---

## Task 6: install.sh arg parsing

**Files:**
- Modify: `docs/public/install.sh`

- [ ] **Step 1: Add arg parsing section**

After the existing environment variable block (lines 9-11), add a CLI arg parser that overrides env vars. Replace the env var block and add parsing:

```bash
# ─── Overridable: environment variables OR CLI flags ─────────────────────────
# --binary / BINARY    Binary to install: lattice (default) or latticed
# --tag / TAG          Specific version e.g. v0.2.0; auto-detected if not set
# --server / SERVER    Control plane URL; triggers non-interactive lattice init
# --token / TOKEN      Enrollment token; triggers non-interactive lattice init
# --install-dir / INSTALL_DIR  Installation directory, default /usr/local/bin
# ─────────────────────────────────────────────────────────────────────────────
BINARY="${BINARY:-lattice}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TAG="${TAG:-}"
SERVER="${SERVER:-}"
TOKEN="${TOKEN:-}"
REPO="alatticeio/lattice"

# Parse CLI flags (override env vars)
while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)   BINARY="$2";      shift 2 ;;
    --tag)      TAG="$2";         shift 2 ;;
    --server)   SERVER="$2";      shift 2 ;;
    --token)    TOKEN="$2";       shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done
```

- [ ] **Step 2: Add auto-enrollment section after binary installation**

After the `echo "✅ $BINARY $TAG installed..."` block at the end of the script, add:

```bash
# ─── Auto-enrollment (when --server and --token are provided) ─────────────────
if [ -n "$SERVER" ] && [ -n "$TOKEN" ]; then
  echo ""
  echo "Auto-enrolling into $SERVER ..."
  "$INSTALL_DIR/$BINARY" init --server "$SERVER" --token "$TOKEN"
  echo "Starting agent..."
  "$INSTALL_DIR/$BINARY" up --detach
  echo ""
  echo "Agent is running. Run '$BINARY status' to check connectivity."
fi
```

- [ ] **Step 3: Test arg parsing locally**

```bash
# Test that flags are parsed correctly (dry run — use a fake server that will fail gracefully)
bash docs/public/install.sh --binary lattice --tag v0.1.0-alpha
# Expected: downloads and installs v0.1.0-alpha, no auto-enrollment since no --server/--token
```

- [ ] **Step 4: Test with pipe (the actual demo invocation)**

```bash
# Test via pipe (how demo modal will call it)
cat docs/public/install.sh | bash -s -- --server https://test.example.com --token lt-fake-xxx
# Expected: downloads binary, runs lattice init --server ... --token ..., then lattice up --detach
# lattice up will fail (no real server) but init should succeed with config written
```

- [ ] **Step 5: Commit**

```bash
git add docs/public/install.sh
git commit -s -m "feat(demo): add --server/--token/--binary/--tag CLI flags to install.sh"
```

---

## Task 7: Frontend DemoModal component

**Files:**
- Create: `frontend/src/components/DemoModal.vue`

- [ ] **Step 1: Check existing Dialog/Modal component imports**

```bash
ls frontend/src/components/ui/dialog/
```

Note the available components (typically `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogFooter`).

- [ ] **Step 2: Create DemoModal.vue**

Create `frontend/src/components/DemoModal.vue`:

```vue
<script setup lang="ts">
import { ref, computed, onUnmounted, watch } from 'vue'
import { Copy, Check, RefreshCw } from 'lucide-vue-next'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface DemoSession {
  workspace_id: string
  expires_at: string
  device1_cmd: string
  device2_cmd: string
}

const STORAGE_KEY = 'lattice_demo'
const emit = defineEmits<{ (e: 'close'): void }>()

defineProps<{ open: boolean }>()

type State = 'loading' | 'ready' | 'expired' | 'error'

const state = ref<State>('loading')
const session = ref<DemoSession | null>(null)
const errorMsg = ref('')
const timeLeft = ref('')
const copied1 = ref(false)
const copied2 = ref(false)

let timer: ReturnType<typeof setInterval> | null = null

function formatTime(ms: number): string {
  if (ms <= 0) return '0:00'
  const totalSec = Math.floor(ms / 1000)
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function startCountdown(expiresAt: string) {
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    const ms = new Date(expiresAt).getTime() - Date.now()
    if (ms <= 0) {
      timeLeft.value = '0:00'
      state.value = 'expired'
      clearInterval(timer!)
    } else {
      timeLeft.value = formatTime(ms)
    }
  }, 1000)
}

async function launch() {
  state.value = 'loading'
  errorMsg.value = ''

  // Check localStorage first
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const cached: DemoSession = JSON.parse(raw)
      if (new Date(cached.expires_at).getTime() > Date.now()) {
        session.value = cached
        state.value = 'ready'
        startCountdown(cached.expires_at)
        return
      }
    }
  } catch {
    // ignore parse errors
  }

  // Call API
  try {
    const res = await fetch('/api/v1/demo/launch', { method: 'POST' })
    if (res.status === 429) {
      errorMsg.value = 'Too many demo sessions from your network. Please try again later.'
      state.value = 'error'
      return
    }
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      errorMsg.value = body.message ?? 'Failed to launch demo. Please try again.'
      state.value = 'error'
      return
    }
    const body = await res.json()
    const data: DemoSession = body.data
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
    session.value = data
    state.value = 'ready'
    startCountdown(data.expires_at)
  } catch {
    errorMsg.value = 'Network error. Please check your connection and try again.'
    state.value = 'error'
  }
}

function reset() {
  localStorage.removeItem(STORAGE_KEY)
  session.value = null
  launch()
}

async function copy(text: string, which: 1 | 2) {
  await navigator.clipboard.writeText(text)
  if (which === 1) {
    copied1.value = true
    setTimeout(() => { copied1.value = false }, 2000)
  } else {
    copied2.value = true
    setTimeout(() => { copied2.value = false }, 2000)
  }
}

watch(() => emit, () => {}, { immediate: false }) // keep emit alive
watch(
  () => ({ openProp: undefined as boolean | undefined }),
  () => {},
  { immediate: true }
)

// Auto-launch when dialog opens
const openModel = defineModel<boolean>('open')
watch(openModel, (v) => {
  if (v) launch()
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <Dialog v-model:open="openModel">
    <DialogContent class="max-w-lg">
      <DialogHeader>
        <DialogTitle>Zero-Friction Demo</DialogTitle>
        <p v-if="state === 'ready' && session" class="text-sm text-muted-foreground mt-1">
          Expires in: <span class="font-mono font-semibold">{{ timeLeft }}</span>
        </p>
      </DialogHeader>

      <!-- Loading -->
      <div v-if="state === 'loading'" class="flex items-center justify-center py-10 text-muted-foreground text-sm">
        Setting up demo workspace...
      </div>

      <!-- Error -->
      <div v-else-if="state === 'error'" class="space-y-4 py-2">
        <p class="text-sm text-destructive">{{ errorMsg }}</p>
        <Button variant="outline" size="sm" @click="launch">Try Again</Button>
      </div>

      <!-- Expired -->
      <div v-else-if="state === 'expired'" class="space-y-4 py-2">
        <p class="text-sm text-muted-foreground">This demo session has expired.</p>
        <Button variant="outline" size="sm" @click="reset">
          <RefreshCw class="mr-2 h-4 w-4" /> Start New Demo
        </Button>
      </div>

      <!-- Ready -->
      <div v-else-if="state === 'ready' && session" class="space-y-5 py-2">
        <div class="space-y-2">
          <p class="text-sm font-medium">Step 1 — Run on Device 1</p>
          <div class="relative rounded-md bg-muted p-3 font-mono text-xs break-all pr-10">
            {{ session.device1_cmd }}
            <Button
              variant="ghost" size="icon"
              class="absolute top-1.5 right-1.5 h-6 w-6"
              @click="copy(session!.device1_cmd, 1)"
            >
              <Check v-if="copied1" class="h-3 w-3 text-green-500" />
              <Copy v-else class="h-3 w-3" />
            </Button>
          </div>
        </div>

        <div class="space-y-2">
          <p class="text-sm font-medium">Step 2 — Run on Device 2</p>
          <div class="relative rounded-md bg-muted p-3 font-mono text-xs break-all pr-10">
            {{ session.device2_cmd }}
            <Button
              variant="ghost" size="icon"
              class="absolute top-1.5 right-1.5 h-6 w-6"
              @click="copy(session!.device2_cmd, 2)"
            >
              <Check v-if="copied2" class="h-3 w-3 text-green-500" />
              <Copy v-else class="h-3 w-3" />
            </Button>
          </div>
        </div>

        <div class="space-y-1">
          <p class="text-sm font-medium">Step 3 — Verify (on either device)</p>
          <div class="rounded-md bg-muted p-3 font-mono text-xs space-y-1">
            <div>lattice status</div>
            <div>ping &lt;peer-ip&gt;</div>
          </div>
        </div>

        <div class="flex justify-between pt-2">
          <Button variant="ghost" size="sm" @click="reset">
            <RefreshCw class="mr-2 h-3 w-3" /> Start New Demo
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
```

- [ ] **Step 3: Build the frontend to verify no compile errors**

```bash
cd frontend && pnpm build
```

Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/DemoModal.vue
git commit -s -m "feat(demo): add DemoModal Vue component with localStorage persistence"
```

---

## Task 8: Frontend landing page button

**Files:**
- Modify: `frontend/src/pages/index.vue`

- [ ] **Step 1: Read current CTA section**

```bash
grep -n "Get Started\|router.push\|handleGetStarted\|cta\|hero" frontend/src/pages/index.vue | head -30
```

Locate the primary CTA button block.

- [ ] **Step 2: Add demo state and import**

In the `<script setup>` block of `index.vue`, add after the existing imports:

```typescript
import DemoModal from '@/components/DemoModal.vue'
```

Add state variables:

```typescript
const demoOpen = ref(false)
const demoEnabled = ref(false)

// Check if demo is enabled on this server
onMounted(async () => {
  // ... (existing onMounted code stays)
  try {
    const r = await fetch('/api/v1/demo/status')
    if (r.ok) {
      const body = await r.json()
      demoEnabled.value = body?.data?.enabled === true
    }
  } catch { /* non-fatal */ }
})
```

Note: merge this with the existing `onMounted` block — don't create a second one. Add the demo status fetch call inside the existing `onMounted`.

- [ ] **Step 3: Add "Try Demo" button to the CTA section**

Find the primary CTA button in the template (look for `@click="router.push('/login')"` or similar). Add the demo button immediately after it:

```html
<Button
  v-if="demoEnabled"
  variant="outline"
  size="lg"
  class="gap-2"
  @click="demoOpen = true"
>
  Try Demo
</Button>
```

- [ ] **Step 4: Add DemoModal to template**

At the very end of the template, just before the closing `</template>`, add:

```html
<DemoModal v-model:open="demoOpen" />
```

- [ ] **Step 5: Build and verify**

```bash
cd frontend && pnpm build
```

Expected: no errors.

- [ ] **Step 6: Run dev server and confirm button appears (with demo enabled)**

```bash
# In one terminal: start latticed with demo enabled
LATTICE_APP_DEMO_ENABLED=true ./bin/latticed

# In another terminal:
cd frontend && pnpm dev
# Open http://localhost:5173 — "Try Demo" button should appear in the CTA section
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/index.vue
git commit -s -m "feat(demo): add Try Demo button to landing page CTA"
```

---

## Self-Review

### Spec coverage check

| Spec requirement | Task |
|------------------|------|
| `POST /api/v1/demo/launch` | Task 3, 4 |
| No auth required | Task 4 (no middleware on the route) |
| Auto-create temp workspace + token | Task 3 |
| Return two curl commands | Task 3 |
| `demo.enabled` flag (default false) | Task 1, 4 |
| IP rate limiting | Task 3, 4 |
| `demo.ttlMinutes` configurable | Task 1, 3 |
| `demo.rateLimitPerHour` configurable | Task 1, 3 |
| Cleanup goroutine every 5 min | Task 3, 4 |
| `is_demo` + `expires_at` on Workspace | Task 1 |
| `ListExpiredDemos` for cleanup | Task 2 |
| install.sh `--server` / `--token` | Task 6 |
| `lattice init --server --token` non-interactive | Task 5 |
| localStorage persistence | Task 7 |
| Countdown timer in modal | Task 7 |
| "Start New Demo" reset | Task 7 |
| Frontend button (disabled when demo off) | Task 8 |
| `GET /api/v1/demo/status` | Task 3, 4 |

All spec requirements covered.

### Known issue to watch for (Task 3, step 4)

The `policyController.CreateOrUpdatePolicy` call in `demo.go` uses a method that may not match the actual `PolicyController` interface. The implementer **must** run `grep -n "interface PolicyController" internal/server/controller/policy.go` and adjust the call to use the correct method name and signature before building.
