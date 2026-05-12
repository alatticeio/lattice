# Agent Isolation Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `AgentIsolationService` and `AgentRegistrationService` into the HTTP server so the full agent enrollment and tool-call isolation flow works end-to-end.

**Architecture:** Add two new service fields to `Server`, initialize them in `NewServer()` when `cfg.AI.AgentIsolation.Enabled`, pass `AgentIsolationService` to both `NewAIServiceWithWorkflow` calls, and expose three new admin routes via a new `agentIsolationRouter()`.

**Tech Stack:** Go 1.25, Gin, controller-runtime client, existing `service.AgentIsolationService` / `service.AgentRegistrationService` interfaces.

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `internal/server/server/server.go` | Add struct fields + init block |
| Modify | `internal/server/server/api.go` | Call `s.agentIsolationRouter()` |
| Create | `internal/server/server/agent_isolation_router.go` | 3 HTTP handlers + `k8sAgentIdentityReader` adapter |

---

### Task 1: Add service fields to Server struct and initialize them

**Files:**
- Modify: `internal/server/server/server.go:76-79` (struct field block near `agentEnrollService`)
- Modify: `internal/server/server/server.go:165-203` (AI service init block)
- Modify: `internal/server/server/server.go:265-270` (Server struct literal)

- [ ] **Step 1: Add two fields to the Server struct**

In `server.go`, find the block ending with `agentEnrollService service.AgentEnrollService` (line 79). Add two fields after it:

```go
	agentEnrollService service.AgentEnrollService

	agentIsolationService service.AgentIsolationService
	agentRegService       service.AgentRegistrationService
```

- [ ] **Step 2: Initialize the services before AI service creation**

In `server.go`, find line 165:
```go
	workflowSvc := service.NewWorkflowService(st)
```

Insert after it (before the `// ── Weak Dependency 3` comment on line 167):

```go
	// ── Agent Isolation (optional; nil-safe when disabled or no k8s client) ──
	var agentIsolSvc service.AgentIsolationService
	var agentRegSvc service.AgentRegistrationService
	if cfg.AI.AgentIsolation.Enabled && client != nil {
		jwtSecret := cfg.AI.AgentIsolation.JWTSecret
		if jwtSecret == "" {
			jwtSecret = cfg.JWT.Secret
		}
		agentIsolSvc = service.NewAgentIsolationService(
			cfg.AI.AgentIsolation,
			&k8sAgentIdentityReader{c: client.Client},
		)
		agentRegSvc = service.NewAgentRegistrationService(jwtSecret, st, client.Client)
		logger.Info("agent isolation enabled", "mode", cfg.AI.AgentIsolation.EnforcementMode)
	}
```

- [ ] **Step 3: Pass agentIsolSvc to both NewAIServiceWithWorkflow calls**

Find the first call (line 175-181):
```go
		aiSvc = service.NewAIServiceWithWorkflow(
			llmClient, st, client, presence,
			cfg.AI.MaxToolCalls,
			workflowSvc,
			cfg.AI.Workflow.AutoApprove,
			nil,
		)
```

Replace the trailing `nil` with `agentIsolSvc`:
```go
		aiSvc = service.NewAIServiceWithWorkflow(
			llmClient, st, client, presence,
			cfg.AI.MaxToolCalls,
			workflowSvc,
			cfg.AI.Workflow.AutoApprove,
			agentIsolSvc,
		)
```

Find the second call (line 201):
```go
	aiSvc = service.NewAIServiceWithWorkflow(nil, st, client, presence, 0, workflowSvc, nil, nil)
```

Replace last `nil` with `agentIsolSvc`:
```go
	aiSvc = service.NewAIServiceWithWorkflow(nil, st, client, presence, 0, workflowSvc, nil, agentIsolSvc)
```

- [ ] **Step 4: Add fields to the Server struct literal**

Find the struct literal block (~line 265-270). After `agentEnrollService: service.NewAgentEnrollService(client),` add:

```go
		agentEnrollService:     service.NewAgentEnrollService(client),
		agentIsolationService:  agentIsolSvc,
		agentRegService:        agentRegSvc,
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/server/server/...
```

Expected: no output (success).

- [ ] **Step 6: Commit**

```bash
cd /Users/francis/workspc/lattice
git add internal/server/server/server.go
git commit -s -m "feat(agent-isolation): wire AgentIsolationService and AgentRegistrationService into Server"
```

---

### Task 2: Create agent_isolation_router.go

**Files:**
- Create: `internal/server/server/agent_isolation_router.go`

- [ ] **Step 1: Create the file**

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
	"net/http"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// agentIsolationRouter registers the Agent Isolation admin API.
// All routes require standard user authentication.
// Routes return 402 when the feature is disabled (agentRegService == nil).
func (s *Server) agentIsolationRouter() {
	g := s.Group("/api/v1/agent-isolation")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	{
		g.POST("/enrollment-tokens", s.handleCreateEnrollmentToken())
		g.POST("/register", s.handleAgentRegister())
		g.DELETE("/agents/:name", s.handleAgentRevoke())
	}
}

// handleCreateEnrollmentToken creates a one-time enrollment token for an agent.
//
// POST /api/v1/agent-isolation/enrollment-tokens
// Body: { "namespace": "default", "allowedTools": ["list_peers"], "ttlSeconds": 3600 }
func (s *Server) handleCreateEnrollmentToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "agent isolation is not enabled"})
			return
		}
		var req struct {
			Namespace    string   `json:"namespace"    binding:"required"`
			AllowedTools []string `json:"allowedTools"`
			TTLSeconds   int      `json:"ttlSeconds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		ttl := time.Duration(req.TTLSeconds) * time.Second
		result, err := s.agentRegService.CreateEnrollmentToken(c.Request.Context(), service.EnrollmentTokenRequest{
			AllowedNamespace: req.Namespace,
			AllowedTools:     req.AllowedTools,
			TTL:              ttl,
		})
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

// handleAgentRegister exchanges an enrollment token for an Agent JWT.
//
// POST /api/v1/agent-isolation/register
// Body: { "enrollmentToken": "...", "agentName": "claude-1", "publicKey": "wg-pubkey" }
func (s *Server) handleAgentRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "agent isolation is not enabled"})
			return
		}
		var req struct {
			EnrollmentToken string `json:"enrollmentToken" binding:"required"`
			AgentName       string `json:"agentName"       binding:"required"`
			PublicKey       string `json:"publicKey"       binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		result, err := s.agentRegService.RegisterAgent(c.Request.Context(), service.AgentRegisterRequest{
			EnrollmentToken: req.EnrollmentToken,
			AgentName:       req.AgentName,
			PublicKey:       req.PublicKey,
		})
		if err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

// handleAgentRevoke revokes an agent by deleting its AgentIdentity CRD.
//
// DELETE /api/v1/agent-isolation/agents/:name?namespace=<ns>
func (s *Server) handleAgentRevoke() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "agent isolation is not enabled"})
			return
		}
		name := c.Param("name")
		namespace := c.Query("namespace")
		if namespace == "" {
			resp.BadRequest(c, "namespace query param is required")
			return
		}
		if err := s.agentRegService.RevokeAgent(c.Request.Context(), namespace, name); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── k8sAgentIdentityReader ────────────────────────────────────────────────────

// k8sAgentIdentityReader adapts a controller-runtime client.Client to
// service.AgentIdentityReader.
type k8sAgentIdentityReader struct {
	c client.Client
}

func (r *k8sAgentIdentityReader) Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentIdentity, error) {
	identity := &v1alpha1.AgentIdentity{}
	if err := r.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, identity); err != nil {
		return nil, err
	}
	return identity, nil
}
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/server/server/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice
git add internal/server/server/agent_isolation_router.go
git commit -s -m "feat(agent-isolation): add agent isolation HTTP routes and k8sAgentIdentityReader adapter"
```

---

### Task 3: Wire agentIsolationRouter into apiRouter

**Files:**
- Modify: `internal/server/server/api.go:116` (after `s.agentRouter()`)

- [ ] **Step 1: Add router call in apiRouter()**

In `api.go`, find:
```go
	s.agentRouter()

	s.platformRouter()
```

Add the new call between them:
```go
	s.agentRouter()

	s.agentIsolationRouter()

	s.platformRouter()
```

- [ ] **Step 2: Build the full server binary to verify**

```bash
cd /Users/francis/workspc/lattice && go build ./cmd/latticed/...
```

Expected: no output (success).

- [ ] **Step 3: Run existing tests**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/... -count=1
```

Expected: all tests pass (PASS).

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice
git add internal/server/server/api.go
git commit -s -m "feat(agent-isolation): register agentIsolationRouter in apiRouter"
```

---

## Manual Smoke Test

After all tasks are done, verify end-to-end with a running `latticed`:

**Config** (`lattice.yaml`):
```yaml
ai:
  agent-isolation:
    enabled: true
    enforcement-mode: audit
    jwt-secret: "test-secret-32-chars-minimum-here"
```

**1. Create enrollment token (admin):**
```bash
curl -s -X POST http://localhost:8080/api/v1/agent-isolation/enrollment-tokens \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{"namespace":"default","allowedTools":["list_peers"],"ttlSeconds":3600}'
# Expected: {"data":{"token":"...","expiresAt":"..."}}
```

**2. Register agent:**
```bash
curl -s -X POST http://localhost:8080/api/v1/agent-isolation/register \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{"enrollmentToken":"<token-from-step-1>","agentName":"test-agent","publicKey":"dummy-key"}'
# Expected: {"data":{"jwt":"eyJ...","agentIdentityName":"test-agent"}}
```

**3. Call a tool as agent:**
```bash
curl -s -X POST http://localhost:8080/api/v1/agents/tools/call \
  -H "Authorization: Bearer <agent-jwt-from-step-2>" \
  -H "Content-Type: application/json" \
  -d '{"workspaceId":"ws-xxx","tool":"list_peers","input":{}}'
# Expected: {"data":{"result":"..."}}
```

**4. Revoke agent:**
```bash
curl -s -X DELETE "http://localhost:8080/api/v1/agent-isolation/agents/test-agent?namespace=default" \
  -H "Authorization: Bearer <admin-jwt>"
# Expected: {"data":null}
```
