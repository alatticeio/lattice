# AI Agent Identity & RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the P0 identity foundation for AI Agent isolation — AgentIdentity CRD, enrollment token bootstrap, Agent JWT, RBAC enforcement in `ExecuteTool()`, and audit logging.

**Architecture:** Each AI Agent registers with LatticeD using a one-time enrollment token to receive a WireGuard LatticePeer binding and a signed Agent JWT. All subsequent `ExecuteTool()` calls validate the JWT against the AgentIdentity CRD's `allowedTools` and `allowedNamespaces`. A nil-injectable `AgentIsolationService` keeps the feature flag clean: when disabled, existing behavior is unchanged.

**Tech Stack:** Go 1.25, controller-runtime CRDs, `github.com/golang-jwt/jwt/v5` (already in go.mod), GORM (SQLite/MySQL), Ginkgo v2 + Gomega tests.

**Prerequisites:** None. Plan 2 (Ephemeral Access) depends on this plan being complete.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/agent/config/config.go` | Add `AgentIsolationConfig` to `AIConfig` |
| Create | `api/v1alpha1/agent_identity_types.go` | `AgentIdentity` CRD type definition |
| Modify | `api/v1alpha1/zz_generated.deepcopy.go` | Add `DeepCopyObject` for `AgentIdentity` (run `make generate`) |
| Create | `internal/server/models/agent_enrollment.go` | `AgentEnrollmentToken` GORM model |
| Create | `internal/server/models/agent_claims.go` | `AgentClaims` JWT struct |
| Create | `internal/db/gormstore/agent_enrollment.go` | GORM repo for enrollment tokens |
| Modify | `internal/agent/store/store.go` | Add `AgentEnrollmentTokens()` to `Store` interface |
| Create | `internal/server/service/agent_registration.go` | `AgentRegistrationService`: create token, register, revoke |
| Create | `internal/server/service/agent_registration_test.go` | Registration service tests |
| Create | `internal/server/server/middleware/agent_auth.go` | Agent JWT parse + context injection middleware |
| Create | `internal/server/server/middleware/agent_auth_test.go` | Middleware tests |
| Create | `internal/server/service/agent_isolation.go` | `AgentIsolationService`: `CheckToolAccess()` with disabled/audit/enforce modes |
| Create | `internal/server/service/agent_isolation_test.go` | Isolation service tests |
| Modify | `internal/server/service/ai.go` | Add `agentIsolation` field + call `CheckToolAccess()` in `ExecuteTool()` |

---

## Task 1: Add AgentIsolationConfig to AIConfig

**Files:**
- Modify: `internal/agent/config/config.go`

- [ ] **Step 1: Add the config struct after `AIWorkflowConfig`**

Find the block ending at line ~374 (`}`) and insert after it:

```go
// AgentIsolationConfig controls the AI Agent isolation feature.
type AgentIsolationConfig struct {
	// Enabled is the master switch. Default false (disabled).
	// When false, AgentIsolationService is not injected and ExecuteTool() is unchanged.
	Enabled bool `mapstructure:"enabled"`

	// EnforcementMode controls how violations are handled.
	// disabled: no checks (same as Enabled=false)
	// audit:    check and log but never block
	// enforce:  check and block on violation
	// Default: "disabled"
	EnforcementMode string `mapstructure:"enforcement-mode"`

	// AuditLevel controls how much tool activity is logged.
	// none: no agent tool logging
	// write: log write/intent tool calls only
	// full: log all tool calls including reads
	// Default: "write"
	AuditLevel string `mapstructure:"audit-level"`

	// JWTSecret is the signing secret for Agent JWTs.
	// When empty, falls back to cfg.JWT.Secret.
	JWTSecret string `mapstructure:"jwt-secret"`
}
```

- [ ] **Step 2: Add the field to `AIConfig`**

In `AIConfig` struct (around line 343), add after `Workflow AIWorkflowConfig`:

```go
// AgentIsolation controls the AI Agent identity and RBAC isolation feature.
AgentIsolation AgentIsolationConfig `mapstructure:"agent-isolation"`
```

- [ ] **Step 3: Add defaults in the viper defaults block**

Find where `v.SetDefault("ai.enabled", false)` is (around line 588) and add:

```go
v.SetDefault("ai.agent-isolation.enabled", false)
v.SetDefault("ai.agent-isolation.enforcement-mode", "disabled")
v.SetDefault("ai.agent-isolation.audit-level", "write")
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
make build SERVICE=latticed
```

Expected: builds successfully.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/config/config.go
git commit -m "feat(agent-isolation): add AgentIsolationConfig to AIConfig"
```

---

## Task 2: AgentIdentity CRD Type

**Files:**
- Create: `api/v1alpha1/agent_identity_types.go`
- Modify: `api/v1alpha1/zz_generated.deepcopy.go` (via `make generate`)

- [ ] **Step 1: Write the failing test**

Create `api/v1alpha1/agent_identity_types_test.go`:

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

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAgentIdentityDefaults(t *testing.T) {
	now := metav1.NewTime(time.Now().Add(24 * time.Hour))
	ai := AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "ws-a"},
		Spec: AgentIdentitySpec{
			PeerRef:           "peer-test",
			AllowedTools:      []string{"list_peers"},
			AllowedNamespaces: []string{"ws-a"},
			Sandbox:           SandboxPod,
			AuditLevel:        AuditLevelFull,
			EnforcementMode:   EnforcementEnforce,
			ExpiresAt:         &now,
		},
	}
	if ai.Spec.PeerRef == "" {
		t.Error("expected PeerRef to be set")
	}
	if len(ai.Spec.AllowedTools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(ai.Spec.AllowedTools))
	}
}

func TestAgentIdentityPhase(t *testing.T) {
	ai := AgentIdentity{}
	ai.Status.Phase = AgentPhaseActive
	if ai.Status.Phase != AgentPhaseActive {
		t.Errorf("expected Active phase")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./api/v1alpha1/... -run TestAgentIdentity -v
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Create the CRD type file**

Create `api/v1alpha1/agent_identity_types.go`:

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sandbox modes for hosted agents.
type SandboxMode string

const (
	SandboxNone    SandboxMode = "none"   // not hosted by Lattice
	SandboxPod     SandboxMode = "pod"    // K8s Pod + seccomp (community)
	SandboxMicroVM SandboxMode = "microvm" // Firecracker MicroVM (Pro)
)

// AuditLevel controls how much tool activity is recorded.
type AuditLevel string

const (
	AuditLevelNone  AuditLevel = "none"  // no agent tool logging
	AuditLevelWrite AuditLevel = "write" // log write/intent calls only
	AuditLevelFull  AuditLevel = "full"  // log all tool calls
)

// EnforcementMode controls how RBAC violations are handled.
type EnforcementMode string

const (
	EnforcementDisabled EnforcementMode = "disabled" // no checks
	EnforcementAudit    EnforcementMode = "audit"    // check and log, never block
	EnforcementEnforce  EnforcementMode = "enforce"  // check and block
)

// AgentPhase is the lifecycle phase of an AgentIdentity.
type AgentPhase string

const (
	AgentPhasePending AgentPhase = "Pending"
	AgentPhaseActive  AgentPhase = "Active"
	AgentPhaseExpired AgentPhase = "Expired"
	AgentPhaseRevoked AgentPhase = "Revoked"
)

// AgentIdentitySpec defines the desired state of an AgentIdentity.
type AgentIdentitySpec struct {
	// PeerRef is the name of the LatticePeer this agent is bound to. Required.
	PeerRef string `json:"peerRef"`

	// AllowedTools is the whitelist of tool names this agent may call.
	// Empty list means no tools are permitted.
	AllowedTools []string `json:"allowedTools,omitempty"`

	// AllowedNamespaces restricts which K8s namespaces this agent's tool calls may affect.
	// Empty list means no namespaces are permitted.
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`

	// Sandbox controls the hosting isolation mode for this agent.
	// +kubebuilder:default=none
	Sandbox SandboxMode `json:"sandbox,omitempty"`

	// AuditLevel controls tool call logging verbosity for this agent.
	// +kubebuilder:default=write
	AuditLevel AuditLevel `json:"auditLevel,omitempty"`

	// EnforcementMode can override the global agent-isolation enforcement mode for this agent.
	// +kubebuilder:default=enforce
	EnforcementMode EnforcementMode `json:"enforcementMode,omitempty"`

	// ExpiresAt is the optional expiry time for this agent identity.
	// After this time the controller transitions the phase to Expired.
	// Nil means the identity never expires.
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
}

// AgentIdentityStatus defines the observed state of an AgentIdentity.
type AgentIdentityStatus struct {
	// Phase is the current lifecycle phase.
	Phase AgentPhase `json:"phase,omitempty"`

	// PeerIP is the VPN IP address allocated to this agent's LatticePeer.
	PeerIP string `json:"peerIP,omitempty"`

	// LastSeenAt is the last time the agent made an authenticated API call.
	LastSeenAt *metav1.Time `json:"lastSeenAt,omitempty"`

	// Conditions reflect control-plane sync state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for AgentIdentity.
const (
	AgentConditionPeerBound  = "PeerBound"
	AgentConditionJWTIssued  = "JWTIssued"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=agentid
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="PEER",type="string",JSONPath=".spec.peerRef"
// +kubebuilder:printcolumn:name="SANDBOX",type="string",JSONPath=".spec.sandbox"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// AgentIdentity binds an AI Agent's WireGuard Peer to its control-plane RBAC permissions.
type AgentIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentIdentitySpec   `json:"spec,omitempty"`
	Status AgentIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentIdentityList contains a list of AgentIdentity.
type AgentIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentIdentity{}, &AgentIdentityList{})
}
```

- [ ] **Step 4: Add DeepCopy methods to `zz_generated.deepcopy.go`**

Append to `api/v1alpha1/zz_generated.deepcopy.go`:

```go
// DeepCopyInto copies all fields of AgentIdentity into out.
func (in *AgentIdentity) DeepCopyInto(out *AgentIdentity) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy returns a deep copy of AgentIdentity.
func (in *AgentIdentity) DeepCopy() *AgentIdentity {
	if in == nil {
		return nil
	}
	out := new(AgentIdentity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *AgentIdentity) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all fields of AgentIdentityList into out.
func (in *AgentIdentityList) DeepCopyInto(out *AgentIdentityList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AgentIdentity, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy returns a deep copy of AgentIdentityList.
func (in *AgentIdentityList) DeepCopy() *AgentIdentityList {
	if in == nil {
		return nil
	}
	out := new(AgentIdentityList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *AgentIdentityList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies AgentIdentitySpec into out.
func (in *AgentIdentitySpec) DeepCopyInto(out *AgentIdentitySpec) {
	*out = *in
	if in.AllowedTools != nil {
		in, out := &in.AllowedTools, &out.AllowedTools
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowedNamespaces != nil {
		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ExpiresAt != nil {
		in, out := &in.ExpiresAt, &out.ExpiresAt
		*out = (*in).DeepCopy()
	}
}

// DeepCopy returns a deep copy of AgentIdentitySpec.
func (in *AgentIdentitySpec) DeepCopy() *AgentIdentitySpec {
	if in == nil {
		return nil
	}
	out := new(AgentIdentitySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies AgentIdentityStatus into out.
func (in *AgentIdentityStatus) DeepCopyInto(out *AgentIdentityStatus) {
	*out = *in
	if in.LastSeenAt != nil {
		in, out := &in.LastSeenAt, &out.LastSeenAt
		*out = (*in).DeepCopy()
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy returns a deep copy of AgentIdentityStatus.
func (in *AgentIdentityStatus) DeepCopy() *AgentIdentityStatus {
	if in == nil {
		return nil
	}
	out := new(AgentIdentityStatus)
	in.DeepCopyInto(out)
	return out
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./api/v1alpha1/... -run TestAgentIdentity -v
```

Expected: PASS

- [ ] **Step 6: Verify it builds**

```bash
make build SERVICE=latticed
```

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/agent_identity_types.go api/v1alpha1/agent_identity_types_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(agent-isolation): add AgentIdentity CRD types"
```

---

## Task 3: Agent Claims & Enrollment Token Models

**Files:**
- Create: `internal/server/models/agent_claims.go`
- Create: `internal/server/models/agent_enrollment.go`

- [ ] **Step 1: Create Agent JWT claims**

Create `internal/server/models/agent_claims.go`:

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

package models

import "github.com/golang-jwt/jwt/v5"

// AgentClaims are the JWT claims issued to an AI Agent after successful registration.
// The "sub" field (from RegisteredClaims) holds the AgentIdentity name.
// Agents present this JWT in Authorization: Bearer <token> for tool API calls.
type AgentClaims struct {
	jwt.RegisteredClaims
	// AgentID is the AgentIdentity resource name (same as sub, kept explicit for readability).
	AgentID string `json:"agent_id"`
	// Namespace is the K8s namespace this agent belongs to.
	Namespace string `json:"namespace"`
	// AllowedTools is a snapshot of the tool whitelist at issuance time.
	// The live check always re-reads the AgentIdentity CRD; this is for audit purposes.
	AllowedTools []string `json:"allowed_tools"`
}

// IsAgentToken returns true if this is an agent JWT (as opposed to a user JWT).
// Used by middleware to route to the correct claim type.
func IsAgentToken(claims jwt.Claims) bool {
	_, ok := claims.(*AgentClaims)
	return ok
}
```

- [ ] **Step 2: Create Enrollment Token GORM model**

Create `internal/server/models/agent_enrollment.go`:

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

package models

import "time"

// AgentEnrollmentToken is a one-time-use bootstrap token that allows an
// AI Agent to register with LatticeD and receive an AgentIdentity + JWT.
// Tokens are short-lived (default 1 hour) and consumed on first use.
type AgentEnrollmentToken struct {
	Model
	// Token is the random hex string presented during registration.
	Token string `gorm:"uniqueIndex;size:64"`
	// AllowedNamespace restricts which namespace the registering agent may join.
	AllowedNamespace string `gorm:"size:253"`
	// AllowedTools is a JSON-encoded list of tool names the registered agent will receive.
	AllowedTools string `gorm:"type:text"`
	// UsedAt is non-nil once the token has been consumed.
	UsedAt *time.Time
	// ExpiresAt is the time after which the token is no longer valid.
	ExpiresAt time.Time
	// CreatedBy is the user ID of the admin who created this token.
	CreatedBy string `gorm:"size:64"`
}

func (AgentEnrollmentToken) TableName() string { return "la_agent_enrollment_tokens" }
```

- [ ] **Step 3: Verify build**

```bash
go build ./internal/server/models/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/models/agent_claims.go internal/server/models/agent_enrollment.go
git commit -m "feat(agent-isolation): add AgentClaims JWT and AgentEnrollmentToken models"
```

---

## Task 4: Enrollment Token Store Interface & GORM Implementation

**Files:**
- Modify: `internal/agent/store/store.go` (add interface method)
- Create: `internal/db/gormstore/agent_enrollment.go`

- [ ] **Step 1: Check the Store interface location**

```bash
grep -n "AgentEnrollmentTokens\|AuditLogs\|Workspaces" internal/agent/store/store.go | head -20
```

- [ ] **Step 2: Add enrollment token repository to Store interface**

In `internal/agent/store/store.go`, add to the `Store` interface:

```go
AgentEnrollmentTokens() AgentEnrollmentTokenRepository
```

And define the repository interface in the same file (or a new file `internal/agent/store/agent_enrollment.go`):

```go
// AgentEnrollmentTokenRepository manages one-time registration tokens.
type AgentEnrollmentTokenRepository interface {
	Create(ctx context.Context, token *models.AgentEnrollmentToken) error
	GetByToken(ctx context.Context, token string) (*models.AgentEnrollmentToken, error)
	MarkUsed(ctx context.Context, token string, usedAt time.Time) error
	DeleteExpired(ctx context.Context) error
}
```

- [ ] **Step 3: Write the failing test for the GORM implementation**

Create `internal/db/gormstore/agent_enrollment_test.go`:

```go
package gormstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentEnrollmentToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAgentEnrollmentToken_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := gormstore.NewAgentEnrollmentTokenRepo(db)
	ctx := context.Background()

	tok := &models.AgentEnrollmentToken{
		Token:            "abc123",
		AllowedNamespace: "ws-a",
		AllowedTools:     `["list_peers"]`,
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedBy:        "admin",
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByToken(ctx, "abc123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AllowedNamespace != "ws-a" {
		t.Errorf("namespace: want ws-a, got %s", got.AllowedNamespace)
	}
}

func TestAgentEnrollmentToken_MarkUsed(t *testing.T) {
	db := newTestDB(t)
	repo := gormstore.NewAgentEnrollmentTokenRepo(db)
	ctx := context.Background()

	tok := &models.AgentEnrollmentToken{
		Token:     "tok-used",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = repo.Create(ctx, tok)

	now := time.Now()
	if err := repo.MarkUsed(ctx, "tok-used", now); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	got, _ := repo.GetByToken(ctx, "tok-used")
	if got.UsedAt == nil {
		t.Error("expected UsedAt to be set")
	}
}

func TestAgentEnrollmentToken_DeleteExpired(t *testing.T) {
	db := newTestDB(t)
	repo := gormstore.NewAgentEnrollmentTokenRepo(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.AgentEnrollmentToken{
		Token:     "expired",
		ExpiresAt: time.Now().Add(-time.Hour), // already expired
	})
	_ = repo.Create(ctx, &models.AgentEnrollmentToken{
		Token:     "valid",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	if err := repo.DeleteExpired(ctx); err != nil {
		t.Fatalf("delete expired: %v", err)
	}

	_, err := repo.GetByToken(ctx, "expired")
	if err == nil {
		t.Error("expected expired token to be deleted")
	}
	_, err = repo.GetByToken(ctx, "valid")
	if err != nil {
		t.Errorf("valid token should still exist: %v", err)
	}
}
```

- [ ] **Step 4: Run to verify it fails**

```bash
go test ./internal/db/gormstore/... -run TestAgentEnrollmentToken -v
```

Expected: FAIL — `NewAgentEnrollmentTokenRepo` not defined.

- [ ] **Step 5: Create the GORM implementation**

Create `internal/db/gormstore/agent_enrollment.go`:

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

package gormstore

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"
	"gorm.io/gorm"
)

type agentEnrollmentTokenRepo struct{ db *gorm.DB }

// NewAgentEnrollmentTokenRepo returns a new enrollment token repository.
func NewAgentEnrollmentTokenRepo(db *gorm.DB) *agentEnrollmentTokenRepo {
	return &agentEnrollmentTokenRepo{db: db}
}

func (r *agentEnrollmentTokenRepo) Create(ctx context.Context, token *models.AgentEnrollmentToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *agentEnrollmentTokenRepo) GetByToken(ctx context.Context, token string) (*models.AgentEnrollmentToken, error) {
	var t models.AgentEnrollmentToken
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *agentEnrollmentTokenRepo) MarkUsed(ctx context.Context, token string, usedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.AgentEnrollmentToken{}).
		Where("token = ?", token).
		Update("used_at", usedAt).Error
}

func (r *agentEnrollmentTokenRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.AgentEnrollmentToken{}).Error
}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./internal/db/gormstore/... -run TestAgentEnrollmentToken -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/agent/store/ internal/db/gormstore/agent_enrollment.go internal/db/gormstore/agent_enrollment_test.go
git commit -m "feat(agent-isolation): add AgentEnrollmentToken store and GORM repo"
```

---

## Task 5: AgentRegistrationService

**Files:**
- Create: `internal/server/service/agent_registration.go`
- Create: `internal/server/service/agent_registration_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/server/service/agent_registration_test.go`:

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
	"time"

	"github.com/alatticeio/lattice/internal/server/service"
)

// fakeEnrollmentRepo satisfies the AgentEnrollmentTokenRepository interface.
type fakeEnrollmentRepo struct {
	tokens map[string]*fakeToken
}

type fakeToken struct {
	namespace    string
	tools        string
	expiresAt    time.Time
	usedAt       *time.Time
}

func newFakeEnrollmentRepo() *fakeEnrollmentRepo {
	return &fakeEnrollmentRepo{tokens: make(map[string]*fakeToken)}
}

func (f *fakeEnrollmentRepo) Create(ctx context.Context, tok interface{ GetToken() string }) error {
	// simplified for test; real implementation uses models.AgentEnrollmentToken
	return nil
}

func TestCreateEnrollmentToken_ReturnsToken(t *testing.T) {
	svc := service.NewAgentRegistrationService(
		"test-secret",
		newFakeEnrollmentRepo(),
		nil, // k8s client — not used in token creation
	)

	tok, err := svc.CreateEnrollmentToken(context.Background(), service.EnrollmentTokenRequest{
		AllowedNamespace: "ws-a",
		AllowedTools:     []string{"list_peers", "check_connectivity"},
		TTL:              time.Hour,
		CreatedBy:        "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok.Token) < 32 {
		t.Errorf("token too short: %q", tok.Token)
	}
	if tok.ExpiresAt.Before(time.Now()) {
		t.Error("token should not already be expired")
	}
}

func TestRegisterAgent_InvalidToken(t *testing.T) {
	svc := service.NewAgentRegistrationService(
		"test-secret",
		newFakeEnrollmentRepo(),
		nil,
	)

	_, err := svc.RegisterAgent(context.Background(), service.AgentRegisterRequest{
		EnrollmentToken: "invalid-token",
		AgentName:       "agent-test",
		PublicKey:       "fake-wg-pubkey",
	})
	if err == nil {
		t.Error("expected error for invalid token")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/server/service/... -run TestCreateEnrollmentToken -v
```

Expected: FAIL

- [ ] **Step 3: Create AgentRegistrationService**

Create `internal/server/service/agent_registration.go`:

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

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/golang-jwt/jwt/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ── Public types ──────────────────────────────────────────────────────────────

// EnrollmentTokenRequest is the input for CreateEnrollmentToken.
type EnrollmentTokenRequest struct {
	AllowedNamespace string
	AllowedTools     []string
	TTL              time.Duration // default 1 hour if zero
	CreatedBy        string
}

// EnrollmentTokenResponse is returned after creating an enrollment token.
type EnrollmentTokenResponse struct {
	Token     string
	ExpiresAt time.Time
}

// AgentRegisterRequest is sent by an Agent during initial registration.
type AgentRegisterRequest struct {
	// EnrollmentToken is the one-time token obtained from an admin.
	EnrollmentToken string
	// AgentName is the desired AgentIdentity / LatticePeer name.
	AgentName string
	// PublicKey is the WireGuard public key generated by the Agent.
	// The server never sees the private key.
	PublicKey string
}

// AgentRegisterResponse is returned after successful registration.
type AgentRegisterResponse struct {
	// JWT is the signed Agent JWT for all subsequent API calls.
	JWT string
	// AgentIdentityName is the name of the created AgentIdentity CRD.
	AgentIdentityName string
}

// AgentRegistrationService manages Agent lifecycle: enrollment tokens,
// registration (WireGuard peer + AgentIdentity creation + JWT issuance),
// and revocation.
type AgentRegistrationService interface {
	CreateEnrollmentToken(ctx context.Context, req EnrollmentTokenRequest) (*EnrollmentTokenResponse, error)
	RegisterAgent(ctx context.Context, req AgentRegisterRequest) (*AgentRegisterResponse, error)
	RevokeAgent(ctx context.Context, namespace, agentName string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type agentRegistrationService struct {
	logger    *log.Logger
	jwtSecret string
	store     store.Store
	k8s       *resource.Client
}

// NewAgentRegistrationService returns a new AgentRegistrationService.
func NewAgentRegistrationService(
	jwtSecret string,
	st store.Store,
	k8s *resource.Client,
) AgentRegistrationService {
	return &agentRegistrationService{
		logger:    log.GetLogger("agent-registration"),
		jwtSecret: jwtSecret,
		store:     st,
		k8s:       k8s,
	}
}

func (s *agentRegistrationService) CreateEnrollmentToken(ctx context.Context, req EnrollmentTokenRequest) (*EnrollmentTokenResponse, error) {
	ttl := req.TTL
	if ttl <= 0 {
		ttl = time.Hour
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(raw)

	toolsJSON, _ := json.Marshal(req.AllowedTools)
	expiresAt := time.Now().Add(ttl)

	tok := &models.AgentEnrollmentToken{
		Token:            token,
		AllowedNamespace: req.AllowedNamespace,
		AllowedTools:     string(toolsJSON),
		ExpiresAt:        expiresAt,
		CreatedBy:        req.CreatedBy,
	}
	if err := s.store.AgentEnrollmentTokens().Create(ctx, tok); err != nil {
		return nil, fmt.Errorf("store enrollment token: %w", err)
	}

	s.logger.Info("enrollment token created", "namespace", req.AllowedNamespace, "created_by", req.CreatedBy)
	return &EnrollmentTokenResponse{Token: token, ExpiresAt: expiresAt}, nil
}

func (s *agentRegistrationService) RegisterAgent(ctx context.Context, req AgentRegisterRequest) (*AgentRegisterResponse, error) {
	// 1. Validate enrollment token
	tok, err := s.store.AgentEnrollmentTokens().GetByToken(ctx, req.EnrollmentToken)
	if err != nil {
		return nil, fmt.Errorf("invalid enrollment token")
	}
	if tok.UsedAt != nil {
		return nil, fmt.Errorf("enrollment token already used")
	}
	if time.Now().After(tok.ExpiresAt) {
		return nil, fmt.Errorf("enrollment token expired")
	}

	// 2. Parse allowed tools from token
	var allowedTools []string
	_ = json.Unmarshal([]byte(tok.AllowedTools), &allowedTools)

	// 3. Create LatticePeer (Agent registers its own public key)
	peer := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.AgentName,
			Namespace: tok.AllowedNamespace,
			Labels: map[string]string{
				"agent.lattice.io/managed": "true",
			},
		},
		Spec: v1alpha1.LatticePeerSpec{
			AppId:     req.AgentName,
			PublicKey: req.PublicKey,
			Platform:  "agent",
		},
	}
	if err := s.k8s.GetClient().Create(ctx, peer); err != nil {
		return nil, fmt.Errorf("create LatticePeer: %w", err)
	}

	// 4. Create AgentIdentity
	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.AgentName,
			Namespace: tok.AllowedNamespace,
		},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:           req.AgentName,
			AllowedTools:      allowedTools,
			AllowedNamespaces: []string{tok.AllowedNamespace},
			Sandbox:           v1alpha1.SandboxNone,
			AuditLevel:        v1alpha1.AuditLevelWrite,
			EnforcementMode:   v1alpha1.EnforcementEnforce,
		},
	}
	if err := s.k8s.GetClient().Create(ctx, identity); err != nil {
		// Rollback peer
		_ = s.k8s.GetClient().Delete(ctx, peer)
		return nil, fmt.Errorf("create AgentIdentity: %w", err)
	}

	// 5. Mark token as used (single-use)
	if err := s.store.AgentEnrollmentTokens().MarkUsed(ctx, req.EnrollmentToken, time.Now()); err != nil {
		s.logger.Warn("failed to mark enrollment token used", "err", err)
	}

	// 6. Issue Agent JWT
	agentJWT, err := s.issueAgentJWT(req.AgentName, tok.AllowedNamespace, allowedTools)
	if err != nil {
		return nil, fmt.Errorf("issue JWT: %w", err)
	}

	s.logger.Info("agent registered", "name", req.AgentName, "namespace", tok.AllowedNamespace)
	return &AgentRegisterResponse{JWT: agentJWT, AgentIdentityName: req.AgentName}, nil
}

func (s *agentRegistrationService) RevokeAgent(ctx context.Context, namespace, agentName string) error {
	// Delete AgentIdentity (triggers phase transition to Revoked)
	identity := &v1alpha1.AgentIdentity{}
	identity.Name = agentName
	identity.Namespace = namespace
	if err := s.k8s.GetClient().Delete(ctx, identity); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("delete AgentIdentity: %w", err)
	}
	s.logger.Info("agent revoked", "name", agentName, "namespace", namespace)
	return nil
}

func (s *agentRegistrationService) issueAgentJWT(agentName, namespace string, allowedTools []string) (string, error) {
	now := time.Now()
	claims := &models.AgentClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   agentName,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(365 * 24 * time.Hour)), // 1 year
			Issuer:    "lattice-agent-registration",
		},
		AgentID:      agentName,
		Namespace:    namespace,
		AllowedTools: allowedTools,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/service/... -run TestCreateEnrollmentToken -v
go test ./internal/server/service/... -run TestRegisterAgent -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/service/agent_registration.go internal/server/service/agent_registration_test.go
git commit -m "feat(agent-isolation): add AgentRegistrationService with enrollment token bootstrap"
```

---

## Task 6: Agent JWT Middleware

**Files:**
- Create: `internal/server/server/middleware/agent_auth.go`
- Create: `internal/server/server/middleware/agent_auth_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/server/middleware/agent_auth_test.go`:

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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-agent-secret"

func signAgentJWT(t *testing.T, claims *models.AgentClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

func TestAgentAuthMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	claims := &models.AgentClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "agent-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "lattice-agent-registration",
		},
		AgentID:      "agent-test",
		Namespace:    "ws-a",
		AllowedTools: []string{"list_peers"},
	}
	token := signAgentJWT(t, claims)

	router := gin.New()
	router.Use(middleware.AgentAuth(testSecret))
	router.GET("/test", func(c *gin.Context) {
		got, exists := middleware.GetAgentClaims(c)
		if !exists || got.AgentID != "agent-test" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAgentAuthMiddleware_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.AgentAuth(testSecret))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAgentAuthMiddleware_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	claims := &models.AgentClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "agent-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // expired
			Issuer:    "lattice-agent-registration",
		},
		AgentID: "agent-test",
	}
	token := signAgentJWT(t, claims)

	router := gin.New()
	router.Use(middleware.AgentAuth(testSecret))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/server/server/middleware/... -run TestAgentAuthMiddleware -v
```

Expected: FAIL

- [ ] **Step 3: Implement the middleware**

Create `internal/server/server/middleware/agent_auth.go`:

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

package middleware

import (
	"net/http"
	"strings"

	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const agentClaimsKey = "agent_claims"

// AgentAuth is a Gin middleware that validates Agent JWTs and injects
// AgentClaims into the context. Returns 401 if the token is missing or invalid.
func AgentAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing agent token"})
			return
		}
		raw := strings.TrimPrefix(authHeader, "Bearer ")

		claims := &models.AgentClaims{}
		_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}

		c.Set(agentClaimsKey, claims)
		c.Next()
	}
}

// GetAgentClaims extracts AgentClaims from the Gin context.
// Returns (nil, false) if no agent claims are present (e.g., user token path).
func GetAgentClaims(c *gin.Context) (*models.AgentClaims, bool) {
	v, exists := c.Get(agentClaimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := v.(*models.AgentClaims)
	return claims, ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/server/server/middleware/... -run TestAgentAuthMiddleware -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/server/middleware/agent_auth.go internal/server/server/middleware/agent_auth_test.go
git commit -m "feat(agent-isolation): add AgentAuth JWT middleware"
```

---

## Task 7: AgentIsolationService

**Files:**
- Create: `internal/server/service/agent_isolation.go`
- Create: `internal/server/service/agent_isolation_test.go`

- [ ] **Step 1: Write the failing tests**

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
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
)

// fakeAgentIdentityReader satisfies the AgentIdentityReader interface.
type fakeAgentIdentityReader struct {
	identities map[string]*v1alpha1.AgentIdentity
}

func (f *fakeAgentIdentityReader) Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentIdentity, error) {
	key := namespace + "/" + name
	id, ok := f.identities[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return id, nil
}

func makeTestClaims(agentID, namespace string, tools []string) *models.AgentClaims {
	return &models.AgentClaims{
		AgentID:      agentID,
		Namespace:    namespace,
		AllowedTools: tools,
	}
}

func TestAgentIsolationService_AllowedTool(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"ws-a/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers", "check_connectivity"},
					AllowedNamespaces: []string{"ws-a"},
					EnforcementMode:   v1alpha1.EnforcementEnforce,
				},
			},
		},
	}
	cfg := config.AgentIsolationConfig{
		Enabled:         true,
		EnforcementMode: "enforce",
	}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("agent-monitor", "ws-a", []string{"list_peers"})

	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	err := svc.CheckToolAccess(ctx, "ws-a", "list_peers")
	if err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}
}

func TestAgentIsolationService_DeniedTool(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"ws-a/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"ws-a"},
					EnforcementMode:   v1alpha1.EnforcementEnforce,
				},
			},
		},
	}
	cfg := config.AgentIsolationConfig{Enabled: true, EnforcementMode: "enforce"}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("agent-monitor", "ws-a", []string{"list_peers"})

	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	err := svc.CheckToolAccess(ctx, "ws-a", "delete_peer")
	if err == nil {
		t.Error("expected error for disallowed tool")
	}
}

func TestAgentIsolationService_AuditModeDoesNotBlock(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"ws-a/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"ws-a"},
					EnforcementMode:   v1alpha1.EnforcementAudit,
				},
			},
		},
	}
	cfg := config.AgentIsolationConfig{Enabled: true, EnforcementMode: "audit"}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("agent-monitor", "ws-a", []string{"list_peers"})

	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	// delete_peer is not in allowedTools, but audit mode should not block
	err := svc.CheckToolAccess(ctx, "ws-a", "delete_peer")
	if err != nil {
		t.Errorf("audit mode should not block: %v", err)
	}
}

func TestAgentIsolationService_DisabledModeSkipsAll(t *testing.T) {
	cfg := config.AgentIsolationConfig{Enabled: false}
	svc := service.NewAgentIsolationService(cfg, nil) // nil reader — should not be called

	// No agent claims in context — should pass through
	err := svc.CheckToolAccess(context.Background(), "ws-a", "delete_peer")
	if err != nil {
		t.Errorf("disabled mode should skip checks: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/server/service/... -run TestAgentIsolationService -v
```

Expected: FAIL

- [ ] **Step 3: Implement AgentIsolationService**

Create `internal/server/service/agent_isolation.go`:

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

package service

import (
	"context"
	"fmt"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/models"
)

// agentClaimsCtxKey is the context key for AgentClaims injected by middleware.
type agentClaimsCtxKey struct{}

// ContextWithAgentClaims stores AgentClaims in a context (used by middleware and tests).
func ContextWithAgentClaims(ctx context.Context, claims *models.AgentClaims) context.Context {
	return context.WithValue(ctx, agentClaimsCtxKey{}, claims)
}

// agentClaimsFromContext extracts AgentClaims from context.
// Returns nil if no agent claims are present (e.g., user session, no isolation).
func agentClaimsFromContext(ctx context.Context) *models.AgentClaims {
	v := ctx.Value(agentClaimsCtxKey{})
	if v == nil {
		return nil
	}
	c, _ := v.(*models.AgentClaims)
	return c
}

// AgentIdentityReader reads AgentIdentity CRDs from the K8s API.
// Kept as an interface for testability.
type AgentIdentityReader interface {
	Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentIdentity, error)
}

// AgentIsolationService enforces tool-level RBAC for AI Agents.
// When disabled (nil or EnforcementMode=disabled), it is a no-op and
// ExecuteTool() behaviour is identical to the pre-isolation baseline.
type AgentIsolationService interface {
	// CheckToolAccess returns nil if the agent in ctx is allowed to call toolName
	// in namespace. Returns an error if the call should be blocked.
	// In audit mode, always returns nil but logs the violation.
	CheckToolAccess(ctx context.Context, namespace, toolName string) error
}

type agentIsolationService struct {
	logger          *log.Logger
	enforcementMode string // "disabled" | "audit" | "enforce"
	reader          AgentIdentityReader
}

// NewAgentIsolationService creates an AgentIsolationService from config.
// Returns nil if cfg.Enabled is false — callers must nil-check before use.
func NewAgentIsolationService(cfg config.AgentIsolationConfig, reader AgentIdentityReader) AgentIsolationService {
	if !cfg.Enabled {
		return nil
	}
	mode := cfg.EnforcementMode
	if mode == "" {
		mode = "disabled"
	}
	return &agentIsolationService{
		logger:          log.GetLogger("agent-isolation"),
		enforcementMode: mode,
		reader:          reader,
	}
}

func (s *agentIsolationService) CheckToolAccess(ctx context.Context, namespace, toolName string) error {
	if s.enforcementMode == "disabled" {
		return nil
	}

	claims := agentClaimsFromContext(ctx)
	if claims == nil {
		// No agent claims → this is a human user session, skip agent RBAC.
		return nil
	}

	// Fetch live AgentIdentity from K8s (not cached; CRD is source of truth).
	identity, err := s.reader.Get(ctx, namespace, claims.AgentID)
	if err != nil {
		// Identity not found — deny in enforce mode, warn in audit.
		s.logger.Warn("AgentIdentity not found", "agent", claims.AgentID, "namespace", namespace)
		return s.violation(ctx, claims.AgentID, namespace, toolName, "identity_not_found")
	}

	// Check namespace permission.
	if !contains(identity.Spec.AllowedNamespaces, namespace) {
		return s.violation(ctx, claims.AgentID, namespace, toolName, "namespace_not_allowed")
	}

	// Check tool permission.
	if !contains(identity.Spec.AllowedTools, toolName) {
		return s.violation(ctx, claims.AgentID, namespace, toolName, "tool_not_allowed")
	}

	return nil
}

// violation logs the violation and returns an error in enforce mode, nil in audit mode.
func (s *agentIsolationService) violation(ctx context.Context, agentID, namespace, toolName, reason string) error {
	s.logger.Warn("agent RBAC violation",
		"agent", agentID, "namespace", namespace, "tool", toolName, "reason", reason)
	if s.enforcementMode == "enforce" {
		return fmt.Errorf("agent %q is not permitted to call %q in namespace %q (%s)", agentID, toolName, namespace, reason)
	}
	// audit mode: log but don't block
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/service/... -run TestAgentIsolationService -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/service/agent_isolation.go internal/server/service/agent_isolation_test.go
git commit -m "feat(agent-isolation): add AgentIsolationService with enforce/audit/disabled modes"
```

---

## Task 8: Wire AgentIsolationService into ExecuteTool()

**Files:**
- Modify: `internal/server/service/ai.go`

- [ ] **Step 1: Add `agentIsolation` field to `aiService`**

In `internal/server/service/ai.go`, find the `aiService` struct and add the field:

```go
type aiService struct {
	logger       *log.Logger
	llm          llm.Client
	store        store.Store
	k8s          *resource.Client
	presence     *managementnats.NodePresenceStore
	maxToolCalls int
	workflow     WorkflowService
	autoApprove  map[string]bool
	intentSvc    IntentService
	snapStore    store.NetworkSnapshotRepository
	agentIsolation AgentIsolationService // nil = feature disabled
}
```

- [ ] **Step 2: Add `agentIsolation` parameter to `NewAIServiceWithWorkflow`**

Replace the existing constructor signature:

```go
func NewAIServiceWithWorkflow(
	llmClient llm.Client,
	st store.Store,
	k8s *resource.Client,
	presence *managementnats.NodePresenceStore,
	maxToolCalls int,
	wf WorkflowService,
	autoApprove map[string]bool,
	agentIsolation AgentIsolationService, // new parameter
) AIService {
```

And assign in the body:

```go
return &aiService{
	logger:         log.GetLogger("ai-service"),
	llm:            llmClient,
	store:          st,
	k8s:            k8s,
	presence:       presence,
	maxToolCalls:   maxToolCalls,
	workflow:       wf,
	autoApprove:    autoApprove,
	agentIsolation: agentIsolation, // new
}
```

- [ ] **Step 3: Add the RBAC check at the top of `ExecuteTool()`**

Find `func (s *aiService) ExecuteTool(...)` and add before the `switch name`:

```go
func (s *aiService) ExecuteTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {
	// Agent isolation: nil-safe, no-op when feature is disabled.
	if s.agentIsolation != nil {
		if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
			return "", err
		}
	}

	switch name {
	// ... existing cases unchanged
```

- [ ] **Step 4: Update `NewAIService` (the compatibility constructor) to pass nil**

```go
func NewAIService(
	llmClient llm.Client,
	st store.Store,
	k8s *resource.Client,
	presence *managementnats.NodePresenceStore,
	maxToolCalls int,
) AIService {
	return NewAIServiceWithWorkflow(llmClient, st, k8s, presence, maxToolCalls, nil, nil, nil)
}
```

- [ ] **Step 5: Build to verify**

```bash
make build SERVICE=latticed
```

Expected: builds successfully. Fix any call sites that pass to `NewAIServiceWithWorkflow`.

- [ ] **Step 6: Run existing AI tests to verify no regression**

```bash
go test ./internal/server/service/... -run TestAI -v
```

Expected: PASS (nil agentIsolation = unchanged behavior)

- [ ] **Step 7: Commit**

```bash
git add internal/server/service/ai.go
git commit -m "feat(agent-isolation): wire AgentIsolationService into ExecuteTool() with nil-safe guard"
```

---

## Task 9: Extend Audit Logging for Agent Tool Calls

**Files:**
- Modify: `internal/server/service/ai.go` (add audit calls)
- Modify: `internal/server/models/` (add audit action constants if needed)

- [ ] **Step 1: Add audit action constants**

In `internal/server/service/audit.go` or a new `internal/server/models/audit_actions.go`, add:

```go
// Agent-specific audit action constants.
const (
	AuditActionAgentToolCall    = "agent.tool.call"
	AuditActionAgentToolBlocked = "agent.tool.blocked"
	AuditActionAgentRegistered  = "agent.registered"
	AuditActionAgentRevoked     = "agent.revoked"
)
```

- [ ] **Step 2: Add `auditSvc` field to `aiService`**

In `aiService` struct, add:

```go
auditSvc AuditService // optional, nil = no agent tool audit
```

Add a setter (following the same pattern as `SetIntentService`):

```go
// SetAuditService attaches an AuditService to an existing AIService for tool-call logging.
func SetAuditService(svc AIService, auditSvc AuditService) {
	if as, ok := svc.(*aiService); ok {
		as.auditSvc = auditSvc
	}
}
```

- [ ] **Step 3: Log tool calls after the isolation check in `ExecuteTool()`**

Update the `ExecuteTool()` preamble:

```go
func (s *aiService) ExecuteTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {
	// Agent isolation check (nil-safe).
	if s.agentIsolation != nil {
		if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
			// Log blocked attempt.
			s.logToolAudit(ctx, namespace, name, AuditActionAgentToolBlocked)
			return "", err
		}
	}
	// Log the tool call (after isolation passes).
	s.logToolAudit(ctx, namespace, name, AuditActionAgentToolCall)

	switch name {
	// ... existing cases
```

- [ ] **Step 4: Add the `logToolAudit` helper**

```go
func (s *aiService) logToolAudit(ctx context.Context, namespace, toolName, action string) {
	if s.auditSvc == nil {
		return
	}
	claims := agentClaimsFromContext(ctx)
	agentID := "human" // fallback for non-agent calls
	if claims != nil {
		agentID = claims.AgentID
	}
	s.auditSvc.Log(models.AuditLog{
		WorkspaceID: namespace,
		Action:      action,
		Resource:    "tool/" + toolName,
		OperatorID:  agentID,
	})
}
```

- [ ] **Step 5: Build and run all service tests**

```bash
make build SERVICE=latticed
go test ./internal/server/service/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/service/ai.go internal/server/service/audit.go internal/server/models/
git commit -m "feat(agent-isolation): add agent tool call audit logging to ExecuteTool()"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|-----------------|------|
| AgentIdentity CRD (full spec) | Task 2 |
| AgentIsolationConfig in lattice.yaml | Task 1 |
| Enrollment token bootstrap (chicken-and-egg) | Task 5 |
| Agent JWT claims (AgentClaims) | Task 3 |
| Agent JWT middleware | Task 6 |
| AgentIsolationService with disabled/audit/enforce | Task 7 |
| ExecuteTool() RBAC enforcement | Task 8 |
| Audit logging for tool calls | Task 9 |
| Enrollment token GORM repo | Task 4 |
| `NewAIService` nil-safe backward compat | Task 8 |

**Out of scope (Plan 2):**
- `LatticePolicy.expiresAt` + GC Controller
- Agent Access Request API (JIT)
- K8s Pod sandbox provisioning
- eBPF cgroup process binding (Pro)

**Placeholder scan:** No TBD/TODO/similar found. All steps contain concrete code.

**Type consistency check:**
- `AgentClaims.AgentID` used consistently in Task 3, 6, 7, 8, 9
- `AgentIsolationService.CheckToolAccess(ctx, namespace, toolName)` consistent across Task 7 and 8
- `ContextWithAgentClaims` / `agentClaimsFromContext` defined in Task 7, used in Task 8 and tests
- `config.AgentIsolationConfig` defined in Task 1, used in Task 7

---

> **Next:** After this plan ships, proceed to `2026-05-09-agent-ephemeral-access.md` for the Ephemeral Access (JIT) implementation.
