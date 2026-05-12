# AgentIdentity Lifecycle Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `AgentIdentity.spec.expiresAt` actually enforced by wiring up a reconciler that marks identities `Expired` and updating `AgentIsolationService` to block expired/revoked agents.

**Architecture:** A new `AgentIdentityReconciler` (in `internal/server/controller/`) watches `AgentIdentity` CRDs, transitions `status.phase` to `Expired` when `spec.expiresAt` passes, and requeues just before expiry. `AgentIsolationService.CheckToolAccess()` gains a phase check that rejects `Expired` and `Revoked` identities. Both the new reconciler and the already-written `AgentTTLReconciler` are registered with the server's controller-runtime manager in `server.go`.

**Tech Stack:** Go, controller-runtime fake client (tests), Gomega assertions (tests)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/server/controller/agent_identity_controller.go` | Reconciles AgentIdentity phase transitions |
| Create | `internal/server/controller/agent_identity_controller_test.go` | Unit tests for the reconciler |
| Modify | `internal/server/service/agent_isolation.go` | Add phase check in `CheckToolAccess` |
| Create | `internal/server/service/agent_isolation_test.go` | Unit tests for isolation service phase check |
| Modify | `internal/server/server/server.go` | Register AgentIdentityReconciler + AgentTTLReconciler with mgr |

---

### Task 1: AgentIdentity Reconciler

**Files:**
- Create: `internal/server/controller/agent_identity_controller.go`

- [ ] **Step 1: Write failing test**

Create `internal/server/controller/agent_identity_controller_test.go`:

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

package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/controller"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestAgentIdentityReconciler_MarksExpired(t *testing.T) {
	g := NewWithT(t)

	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "claude-1",
			Namespace: "default",
		},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:         "claude-1",
			AllowedTools:    []string{"list_peers"},
			EnforcementMode: v1alpha1.EnforcementEnforce,
			ExpiresAt:       &metav1.Time{Time: time.Now().Add(-time.Minute)},
		},
		Status: v1alpha1.AgentIdentityStatus{
			Phase: v1alpha1.AgentPhaseActive,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&v1alpha1.AgentIdentity{}).
		Build()

	r := controller.NewAgentIdentityReconciler(fakeClient)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "claude-1"},
	})
	g.Expect(err).ToNot(HaveOccurred())

	var got v1alpha1.AgentIdentity
	g.Expect(fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claude-1"}, &got)).To(Succeed())
	g.Expect(got.Status.Phase).To(Equal(v1alpha1.AgentPhaseExpired))
}

func TestAgentIdentityReconciler_RequeuesBeforeExpiry(t *testing.T) {
	g := NewWithT(t)

	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "claude-2",
			Namespace: "default",
		},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:      "claude-2",
			AllowedTools: []string{"list_peers"},
			ExpiresAt:    &metav1.Time{Time: time.Now().Add(time.Hour)},
		},
		Status: v1alpha1.AgentIdentityStatus{Phase: v1alpha1.AgentPhaseActive},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&v1alpha1.AgentIdentity{}).
		Build()

	r := controller.NewAgentIdentityReconciler(fakeClient)
	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "claude-2"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeNumerically(">", 0))

	var got v1alpha1.AgentIdentity
	g.Expect(fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claude-2"}, &got)).To(Succeed())
	g.Expect(got.Status.Phase).To(Equal(v1alpha1.AgentPhaseActive), "should not change phase for live identity")
}

func TestAgentIdentityReconciler_NoExpiry_NoOp(t *testing.T) {
	g := NewWithT(t)

	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "claude-3", Namespace: "default"},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:      "claude-3",
			AllowedTools: []string{"list_peers"},
		},
		Status: v1alpha1.AgentIdentityStatus{Phase: v1alpha1.AgentPhaseActive},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&v1alpha1.AgentIdentity{}).
		Build()

	r := controller.NewAgentIdentityReconciler(fakeClient)
	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "claude-3"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/francis/workspc/lattice
go test ./internal/server/controller/... -run "TestAgentIdentityReconciler" -v 2>&1 | head -20
```

Expected: FAIL with `undefined: controller.NewAgentIdentityReconciler`

- [ ] **Step 3: Implement the reconciler**

Create `internal/server/controller/agent_identity_controller.go`:

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

package controller

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AgentIdentityReconciler transitions AgentIdentity phase to Expired when spec.expiresAt passes.
type AgentIdentityReconciler struct {
	client client.Client
	logger *log.Logger
}

// NewAgentIdentityReconciler creates a reconciler using the provided client directly.
// Useful for testing without a manager.
func NewAgentIdentityReconciler(c client.Client) *AgentIdentityReconciler {
	return &AgentIdentityReconciler{client: c, logger: log.GetLogger("agent-identity")}
}

func (r *AgentIdentityReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var identity v1alpha1.AgentIdentity
	if err := r.client.Get(ctx, req.NamespacedName, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Already in a terminal phase — nothing to do.
	if identity.Status.Phase == v1alpha1.AgentPhaseExpired ||
		identity.Status.Phase == v1alpha1.AgentPhaseRevoked {
		return reconcile.Result{}, nil
	}

	if identity.Spec.ExpiresAt == nil {
		return reconcile.Result{}, nil // no TTL set
	}

	now := time.Now()
	expiresAt := identity.Spec.ExpiresAt.Time

	if now.After(expiresAt) {
		patch := client.MergeFrom(identity.DeepCopy())
		identity.Status.Phase = v1alpha1.AgentPhaseExpired
		if err := r.client.Status().Patch(ctx, &identity, patch); err != nil {
			return reconcile.Result{}, err
		}
		r.logger.Info("agent identity expired", "name", identity.Name, "namespace", identity.Namespace)
		return reconcile.Result{}, nil
	}

	// Requeue just after the expiry time.
	requeueAfter := expiresAt.Sub(now) + time.Second
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager registers the reconciler with a controller-runtime manager.
func (r *AgentIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.logger = log.GetLogger("agent-identity")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AgentIdentity{}).
		Complete(r)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/francis/workspc/lattice
go test ./internal/server/controller/... -run "TestAgentIdentityReconciler" -v
```

Expected: 3 tests PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice
git add internal/server/controller/agent_identity_controller.go \
        internal/server/controller/agent_identity_controller_test.go
git commit -s -m "feat(agent-isolation): add AgentIdentityReconciler for phase TTL management"
```

---

### Task 2: Enforce Phase in AgentIsolationService

**Files:**
- Modify: `internal/server/service/agent_isolation.go`
- Create: `internal/server/service/agent_isolation_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/server/service/agent_isolation_test.go`:

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

package service_test

import (
	"context"
	"testing"

	"github.com/alatticeio/lattice/api/v1alpha1"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	. "github.com/onsi/gomega"
)

// stubReader is a test double for AgentIdentityReader.
type stubReader struct {
	identity *v1alpha1.AgentIdentity
	err      error
}

func (s *stubReader) Get(_ context.Context, _, _ string) (*v1alpha1.AgentIdentity, error) {
	return s.identity, s.err
}

func makeEnforceConfig() agentconfig.AgentIsolationConfig {
	return agentconfig.AgentIsolationConfig{
		Enabled:         true,
		EnforcementMode: "enforce",
	}
}

func agentCtx(agentID, namespace string, tools []string) context.Context {
	return service.ContextWithAgentClaims(context.Background(), &models.AgentClaims{
		AgentID:      agentID,
		Namespace:    namespace,
		AllowedTools: tools,
	})
}

func TestCheckToolAccess_BlocksExpiredIdentity(t *testing.T) {
	g := NewWithT(t)

	reader := &stubReader{
		identity: &v1alpha1.AgentIdentity{
			Spec: v1alpha1.AgentIdentitySpec{
				AllowedTools:      []string{"list_peers"},
				AllowedNamespaces: []string{"default"},
			},
			Status: v1alpha1.AgentIdentityStatus{
				Phase: v1alpha1.AgentPhaseExpired,
			},
		},
	}

	svc := service.NewAgentIsolationService(makeEnforceConfig(), reader)
	ctx := agentCtx("claude-1", "default", []string{"list_peers"})

	err := svc.CheckToolAccess(ctx, "default", "list_peers")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expired"))
}

func TestCheckToolAccess_BlocksRevokedIdentity(t *testing.T) {
	g := NewWithT(t)

	reader := &stubReader{
		identity: &v1alpha1.AgentIdentity{
			Spec: v1alpha1.AgentIdentitySpec{
				AllowedTools:      []string{"list_peers"},
				AllowedNamespaces: []string{"default"},
			},
			Status: v1alpha1.AgentIdentityStatus{
				Phase: v1alpha1.AgentPhaseRevoked,
			},
		},
	}

	svc := service.NewAgentIsolationService(makeEnforceConfig(), reader)
	ctx := agentCtx("claude-1", "default", []string{"list_peers"})

	err := svc.CheckToolAccess(ctx, "default", "list_peers")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("revoked"))
}

func TestCheckToolAccess_AllowsActiveIdentity(t *testing.T) {
	g := NewWithT(t)

	reader := &stubReader{
		identity: &v1alpha1.AgentIdentity{
			Spec: v1alpha1.AgentIdentitySpec{
				AllowedTools:      []string{"list_peers"},
				AllowedNamespaces: []string{"default"},
			},
			Status: v1alpha1.AgentIdentityStatus{
				Phase: v1alpha1.AgentPhaseActive,
			},
		},
	}

	svc := service.NewAgentIsolationService(makeEnforceConfig(), reader)
	ctx := agentCtx("claude-1", "default", []string{"list_peers"})

	err := svc.CheckToolAccess(ctx, "default", "list_peers")
	g.Expect(err).ToNot(HaveOccurred())
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/francis/workspc/lattice
go test ./internal/server/service/... -run "TestCheckToolAccess" -v 2>&1 | head -30
```

Expected: FAIL — `TestCheckToolAccess_BlocksExpiredIdentity` and `TestCheckToolAccess_BlocksRevokedIdentity` should FAIL (no phase check yet), `TestCheckToolAccess_AllowsActiveIdentity` should PASS.

- [ ] **Step 3: Add phase check to CheckToolAccess**

In `internal/server/service/agent_isolation.go`, modify the `CheckToolAccess` method. After fetching the identity (line 97), add the phase check before the namespace/tool checks:

```go
func (s *agentIsolationService) CheckToolAccess(ctx context.Context, namespace, toolName string) error {
	if s.enforcementMode == "disabled" {
		return nil
	}

	claims := agentClaimsFromContext(ctx)
	if claims == nil {
		return nil
	}

	// Fetch live AgentIdentity from K8s (not cached; CRD is source of truth).
	identity, err := s.reader.Get(ctx, namespace, claims.AgentID)
	if err != nil {
		s.logger.Warn("AgentIdentity not found", "agent", claims.AgentID, "namespace", namespace)
		return s.violation(claims.AgentID, namespace, toolName, "identity_not_found")
	}

	// Reject terminal phases before checking tools/namespaces.
	switch identity.Status.Phase {
	case v1alpha1.AgentPhaseExpired:
		return s.violation(claims.AgentID, namespace, toolName, "expired")
	case v1alpha1.AgentPhaseRevoked:
		return s.violation(claims.AgentID, namespace, toolName, "revoked")
	}

	// Check namespace permission.
	if !containsStr(identity.Spec.AllowedNamespaces, namespace) {
		return s.violation(claims.AgentID, namespace, toolName, "namespace_not_allowed")
	}

	// Check tool permission.
	if !containsStr(identity.Spec.AllowedTools, toolName) {
		return s.violation(claims.AgentID, namespace, toolName, "tool_not_allowed")
	}

	return nil
}
```

Also add `v1alpha1` to the import block (it is not currently imported):

```go
import (
	"context"
	"fmt"

	"github.com/alatticeio/lattice/api/v1alpha1"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/models"
)
```

And update the `violation` helper so "expired"/"revoked" produce clear messages. The existing `violation` method already formats `reason` into the message, so the test `ContainSubstring("expired")` will match via `"agent ... (expired)"`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/francis/workspc/lattice
go test ./internal/server/service/... -run "TestCheckToolAccess" -v
```

Expected: 3 tests PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/francis/workspc/lattice
git add internal/server/service/agent_isolation.go \
        internal/server/service/agent_isolation_test.go
git commit -s -m "feat(agent-isolation): reject Expired/Revoked identities in CheckToolAccess"
```

---

### Task 3: Register Reconcilers in server.go

**Files:**
- Modify: `internal/server/server/server.go`

The server already has a manager (`mgr`) and registers `SnapshotController` inside the `if cfg.AI.Enabled && cfg.AI.APIKey != ""` block. Both `AgentTTLReconciler` and `AgentIdentityReconciler` should be registered whenever agent isolation is enabled (not just when AI is enabled), so they go in the `if cfg.AI.AgentIsolation.Enabled && client != nil` block.

- [ ] **Step 1: Add reconciler registration**

In `internal/server/server/server.go`, inside the block at line ~174 (`if cfg.AI.AgentIsolation.Enabled && client != nil {`), after line 184 (`logger.Info("agent isolation enabled", ...)`), add:

```go
		if mgr != nil {
			if err := controller.NewAgentTTLReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
				logger.Warn("failed to setup AgentTTLReconciler", "err", err)
			}
			if err := controller.NewAgentIdentityReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
				logger.Warn("failed to setup AgentIdentityReconciler", "err", err)
			}
		}
```

Ensure `controller` import is present — `"github.com/alatticeio/lattice/internal/server/controller"` is already imported in `server.go` (it is used for `controller.NewSnapshotController`).

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /Users/francis/workspc/lattice
mkdir -p internal/web/dist && touch internal/web/dist/.gitkeep
go build ./internal/... ./cmd/...
```

Expected: exits 0, no errors.

- [ ] **Step 3: Run all tests**

```bash
cd /Users/francis/workspc/lattice
go test ./internal/server/... -count=1 -timeout 3m
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice
git add internal/server/server/server.go
git commit -s -m "feat(agent-isolation): register AgentTTLReconciler and AgentIdentityReconciler with manager"
```

---

### Task 4: Full suite + lint

- [ ] **Step 1: Run full test suite**

```bash
cd /Users/francis/workspc/lattice
mkdir -p internal/web/dist && touch internal/web/dist/.gitkeep
go test ./internal/... ./pkg/... -count=1 -timeout 5m
```

Expected: all PASS

- [ ] **Step 2: Lint**

```bash
cd /Users/francis/workspc/lattice
make lint
```

Expected: no new lint errors. If `errcheck` complains about the `_ = json.Unmarshal` pattern already in the codebase, ignore (it's pre-existing).

- [ ] **Step 3: Push branch**

```bash
cd /Users/francis/workspc/lattice
git push
```

---

## Self-Review Checklist

- **Spec coverage**
  - `spec.expiresAt` triggers phase → Expired: ✅ Task 1
  - `CheckToolAccess` rejects Expired/Revoked: ✅ Task 2
  - Reconcilers wired into running server: ✅ Task 3
  - `AgentTTLReconciler` (was unregistered): ✅ Task 3

- **Placeholder scan**: No TBDs. All code is complete.

- **Type consistency**:
  - `AgentPhaseExpired` / `AgentPhaseRevoked` — defined in `api/v1alpha1/agent_identity_types.go`
  - `NewAgentIdentityReconciler` — defined in Task 1, used in Task 3
  - `NewAgentTTLReconciler` — already exists in `internal/server/controller/agent_ttl_controller.go`
