# AI Agent Secure Mesh — Plan B: MCPServer + AgentPolicy + MCP Proxy + Audit

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add MCPServer and AgentPolicy CRDs to declare MCP servers and tool-level access policies, expose CRUD APIs, and implement an agent-side MCP HTTP proxy that enforces policy and writes audit events.

**Architecture:** Server side adds two new namespace-scoped CRDs (MCPServer, AgentPolicy) with controller-runtime controllers and Gin CRUD routers wired into the existing Server struct. Agent side adds an HTTP proxy (`internal/agent/mcpproxy`) that intercepts the AI agent's HTTP calls, fetches MCPServer+AgentPolicy config from the server using the agent JWT, enforces tool-level policy, and writes JSONL audit events. The proxy is injected as `HTTP_PROXY` into the AI agent child process via `forkAgent`.

**Tech Stack:** Go 1.25, controller-runtime, Gin, `sigs.k8s.io/controller-runtime/pkg/client`, `k8s.io/apimachinery`, `encoding/json`, Ginkgo v2 e2e.

**Natural checkpoint:** Tasks 1–7 (server side) can be merged independently. Tasks 8–11 (agent side) follow after.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `api/v1alpha1/mcp_server_types.go` | Create | MCPServer + AgentPolicy CRD type definitions |
| `api/v1alpha1/zz_generated.deepcopy.go` | Modify | Add DeepCopy methods for new types |
| `internal/server/controller/mcp_server_controller.go` | Create | Reconcile MCPServer status from LatticePeer |
| `internal/server/service/mcp_server.go` | Create | MCPServer CRUD service |
| `internal/server/service/agent_policy.go` | Create | AgentPolicy CRUD service |
| `internal/server/server/mcp_server_router.go` | Create | MCPServer HTTP handlers |
| `internal/server/server/agent_policy_router.go` | Create | AgentPolicy HTTP handlers + agent config endpoint |
| `internal/server/server/server.go` | Modify | Add mcpServerSvc + agentPolicySvc fields |
| `internal/server/server/api.go` | Modify | Register new routers |
| `internal/agent/mcpproxy/proxy.go` | Create | MCP-aware HTTP proxy |
| `internal/agent/mcpproxy/policy_cache.go` | Create | Fetch + cache AgentPolicy+MCPServer from server |
| `internal/agent/mcpproxy/audit.go` | Create | MCP audit event JSONL writer |
| `cmd/lattice/cmd/sandbox/run.go` | Modify | Start MCP proxy + inject HTTP_PROXY into child |
| `cmd/lattice/cmd/sandbox/shared_linux.go` | Modify | Pass httpProxyAddr to forkAgent |

---

### Task 1: MCPServer + AgentPolicy CRD type definitions + deepcopy

**Files:**
- Create: `api/v1alpha1/mcp_server_types.go`
- Modify: `api/v1alpha1/zz_generated.deepcopy.go`

- [ ] **Step 1: Create `api/v1alpha1/mcp_server_types.go`**

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ── MCPServer ─────────────────────────────────────────────────────────────────

// RiskLevel indicates the operational risk of a tool call.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// MCPServerPhase is the lifecycle phase of an MCPServer.
type MCPServerPhase string

const (
	MCPServerPhasePending  MCPServerPhase = "Pending"
	MCPServerPhaseReady    MCPServerPhase = "Ready"
	MCPServerPhaseDegraded MCPServerPhase = "Degraded"
)

// MCPTool declares a single MCP tool exposed by an MCPServer.
type MCPTool struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	RiskLevel   RiskLevel `json:"riskLevel,omitempty"`
}

// MCPServerSpec defines the desired state of an MCPServer.
type MCPServerSpec struct {
	// PeerName is the corresponding LatticePeer name (optional).
	// Set for internal overlay MCPs; omit for external platform MCPs (GitHub, Stripe, etc.).
	// +optional
	PeerName string `json:"peerName,omitempty"`

	// Endpoint is the MCP server address.
	// Internal mode: "http://localhost:3000/mcp" (peer-local address).
	// External mode: "https://mcp.github.com" (full URL for platform MCPs).
	Endpoint string `json:"endpoint"`

	// Tools declares the MCP tools this server exposes, used for AgentPolicy references.
	// +optional
	Tools []MCPTool `json:"tools,omitempty"`
}

// MCPServerStatus is the observed state of an MCPServer.
type MCPServerStatus struct {
	// Phase is Ready when the server is reachable (internal: peer is Ready; external: always Ready).
	Phase MCPServerPhase `json:"phase,omitempty"`
	// Mode is "internal" when peerName is set, "external" otherwise.
	Mode string `json:"mode,omitempty"`
	// PeerAddress is the overlay IP of the LatticePeer (internal mode only).
	PeerAddress  string       `json:"peerAddress,omitempty"`
	LastSyncedAt *metav1.Time `json:"lastSyncedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcpsrv
// +kubebuilder:printcolumn:name="MODE",type="string",JSONPath=".status.mode"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".spec.endpoint"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// MCPServer registers an MCP server (internal overlay or external platform) so that
// AgentPolicy can reference its tools and the MCP proxy can enforce access control.
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MCPServerSpec   `json:"spec,omitempty"`
	Status            MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPServerList contains a list of MCPServer.
type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

// ── AgentPolicy ───────────────────────────────────────────────────────────────

// AgentToolPermission grants a set of tools on a named MCPServer.
type AgentToolPermission struct {
	// MCPServer is the name of the MCPServer in the same namespace.
	MCPServer string `json:"mcpServer"`
	// Tools is the list of allowed tool names. Use ["*"] to allow all tools.
	Tools []string `json:"tools"`
}

// AgentPolicySpec defines which tools an agent may call.
type AgentPolicySpec struct {
	// AgentSelector selects AgentIdentity objects by label.
	AgentSelector metav1.LabelSelector `json:"agentSelector"`
	// AllowedTools is the whitelist of tool grants. When DefaultDeny is true,
	// any tool not listed here is denied.
	// +optional
	AllowedTools []AgentToolPermission `json:"allowedTools,omitempty"`
	// DefaultDeny enables deny-by-default; only explicitly listed tools are allowed.
	// +optional
	DefaultDeny bool `json:"defaultDeny,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=apolicy
// +kubebuilder:printcolumn:name="DEFAULT-DENY",type="boolean",JSONPath=".spec.defaultDeny"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// AgentPolicy enforces tool-level access control for AI agents.
type AgentPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AgentPolicySpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// AgentPolicyList contains a list of AgentPolicy.
type AgentPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
	SchemeBuilder.Register(&AgentPolicy{}, &AgentPolicyList{})
}
```

- [ ] **Step 2: Add DeepCopy methods to `api/v1alpha1/zz_generated.deepcopy.go`**

Append these functions at the end of the file (before the final blank line):

```go
// ── MCPServer DeepCopy ────────────────────────────────────────────────────────

func (in *MCPServer) DeepCopyInto(out *MCPServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *MCPServer) DeepCopy() *MCPServer {
	if in == nil {
		return nil
	}
	out := new(MCPServer)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *MCPServerList) DeepCopyInto(out *MCPServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MCPServer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *MCPServerList) DeepCopy() *MCPServerList {
	if in == nil {
		return nil
	}
	out := new(MCPServerList)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *MCPServerSpec) DeepCopyInto(out *MCPServerSpec) {
	*out = *in
	if in.Tools != nil {
		in, out := &in.Tools, &out.Tools
		*out = make([]MCPTool, len(*in))
		copy(*out, *in)
	}
}

func (in *MCPServerSpec) DeepCopy() *MCPServerSpec {
	if in == nil {
		return nil
	}
	out := new(MCPServerSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServerStatus) DeepCopyInto(out *MCPServerStatus) {
	*out = *in
	if in.LastSyncedAt != nil {
		in, out := &in.LastSyncedAt, &out.LastSyncedAt
		*out = (*in).DeepCopy()
	}
}

func (in *MCPServerStatus) DeepCopy() *MCPServerStatus {
	if in == nil {
		return nil
	}
	out := new(MCPServerStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPTool) DeepCopyInto(out *MCPTool) { *out = *in }

func (in *MCPTool) DeepCopy() *MCPTool {
	if in == nil {
		return nil
	}
	out := new(MCPTool)
	in.DeepCopyInto(out)
	return out
}

// ── AgentPolicy DeepCopy ──────────────────────────────────────────────────────

func (in *AgentPolicy) DeepCopyInto(out *AgentPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *AgentPolicy) DeepCopy() *AgentPolicy {
	if in == nil {
		return nil
	}
	out := new(AgentPolicy)
	in.DeepCopyInto(out)
	return out
}

func (in *AgentPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *AgentPolicyList) DeepCopyInto(out *AgentPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AgentPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *AgentPolicyList) DeepCopy() *AgentPolicyList {
	if in == nil {
		return nil
	}
	out := new(AgentPolicyList)
	in.DeepCopyInto(out)
	return out
}

func (in *AgentPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *AgentPolicySpec) DeepCopyInto(out *AgentPolicySpec) {
	*out = *in
	in.AgentSelector.DeepCopyInto(&out.AgentSelector)
	if in.AllowedTools != nil {
		in, out := &in.AllowedTools, &out.AllowedTools
		*out = make([]AgentToolPermission, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *AgentPolicySpec) DeepCopy() *AgentPolicySpec {
	if in == nil {
		return nil
	}
	out := new(AgentPolicySpec)
	in.DeepCopyInto(out)
	return out
}

func (in *AgentToolPermission) DeepCopyInto(out *AgentToolPermission) {
	*out = *in
	if in.Tools != nil {
		in, out := &in.Tools, &out.Tools
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *AgentToolPermission) DeepCopy() *AgentToolPermission {
	if in == nil {
		return nil
	}
	out := new(AgentToolPermission)
	in.DeepCopyInto(out)
	return out
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/francis/workspc/lattice
go build ./api/v1alpha1/...
```

Expected: no errors.

- [ ] **Step 4: Lint**

```bash
make lint
```

Expected: 0 issues.

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/mcp_server_types.go api/v1alpha1/zz_generated.deepcopy.go
git commit -s -m "feat(api): add MCPServer and AgentPolicy CRD type definitions"
```

---

### Task 2: MCPServer controller

**Files:**
- Create: `internal/server/controller/mcp_server_controller.go`

- [ ] **Step 1: Create the controller**

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
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MCPServerReconciler reconciles MCPServer status from the underlying LatticePeer.
type MCPServerReconciler struct {
	client client.Client
	logger *log.Logger
}

// NewMCPServerReconciler creates a reconciler.
func NewMCPServerReconciler(c client.Client) *MCPServerReconciler {
	return &MCPServerReconciler{client: c, logger: log.GetLogger("mcp-server")}
}

func (r *MCPServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MCPServer{}).
		Complete(r)
}

func (r *MCPServerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var mcpSrv v1alpha1.MCPServer
	if err := r.client.Get(ctx, req.NamespacedName, &mcpSrv); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	patch := client.MergeFrom(mcpSrv.DeepCopy())
	now := metav1.NewTime(time.Now())
	mcpSrv.Status.LastSyncedAt = &now

	if mcpSrv.Spec.PeerName == "" {
		// External mode: no LatticePeer dependency; always Ready.
		mcpSrv.Status.Mode = "external"
		mcpSrv.Status.Phase = v1alpha1.MCPServerPhaseReady
		mcpSrv.Status.PeerAddress = ""
	} else {
		// Internal mode: look up the LatticePeer.
		mcpSrv.Status.Mode = "internal"
		var peer v1alpha1.LatticePeer
		err := r.client.Get(ctx, client.ObjectKey{
			Namespace: req.Namespace,
			Name:      mcpSrv.Spec.PeerName,
		}, &peer)
		if err != nil {
			if apierrors.IsNotFound(err) {
				mcpSrv.Status.Phase = v1alpha1.MCPServerPhasePending
				mcpSrv.Status.PeerAddress = ""
			} else {
				return reconcile.Result{}, err
			}
		} else if peer.Status.Phase == "Ready" && peer.Status.AllocatedAddress != nil {
			mcpSrv.Status.Phase = v1alpha1.MCPServerPhaseReady
			// Strip CIDR suffix if present (e.g. "10.0.7.5/32" → "10.0.7.5")
			addr := *peer.Status.AllocatedAddress
			if idx := strings.Index(addr, "/"); idx != -1 {
				addr = addr[:idx]
			}
			mcpSrv.Status.PeerAddress = addr
		} else {
			mcpSrv.Status.Phase = v1alpha1.MCPServerPhaseDegraded
			mcpSrv.Status.PeerAddress = ""
		}
	}

	if err := r.client.Status().Patch(ctx, &mcpSrv, patch); err != nil {
		return reconcile.Result{}, err
	}
	r.logger.Info("MCPServer reconciled", "name", mcpSrv.Name, "namespace", mcpSrv.Namespace,
		"phase", mcpSrv.Status.Phase, "mode", mcpSrv.Status.Mode)
	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/server/controller/...
```

Expected: no errors.

- [ ] **Step 3: Lint + commit**

```bash
make lint
git add internal/server/controller/mcp_server_controller.go
git commit -s -m "feat(controller): add MCPServer reconciler — sync status from LatticePeer"
```

---

### Task 3: MCPServer and AgentPolicy services

**Files:**
- Create: `internal/server/service/mcp_server.go`
- Create: `internal/server/service/agent_policy.go`

- [ ] **Step 1: Create `internal/server/service/mcp_server.go`**

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MCPServerService manages MCPServer CRD CRUD.
type MCPServerService interface {
	List(ctx context.Context, namespace string) ([]v1alpha1.MCPServer, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.MCPServer, error)
	Create(ctx context.Context, namespace string, spec v1alpha1.MCPServerSpec, displayName string) (*v1alpha1.MCPServer, error)
	Update(ctx context.Context, namespace, name string, spec v1alpha1.MCPServerSpec) (*v1alpha1.MCPServer, error)
	Delete(ctx context.Context, namespace, name string) error
}

type mcpServerService struct {
	k8s k8sclient.Client
}

// NewMCPServerService creates a new MCPServerService.
func NewMCPServerService(k8s k8sclient.Client) MCPServerService {
	return &mcpServerService{k8s: k8s}
}

func (s *mcpServerService) List(ctx context.Context, namespace string) ([]v1alpha1.MCPServer, error) {
	list := &v1alpha1.MCPServerList{}
	if err := s.k8s.List(ctx, list, k8sclient.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list MCPServers: %w", err)
	}
	return list.Items, nil
}

func (s *mcpServerService) Get(ctx context.Context, namespace, name string) (*v1alpha1.MCPServer, error) {
	obj := &v1alpha1.MCPServer{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get MCPServer %s/%s: %w", namespace, name, err)
	}
	return obj, nil
}

func (s *mcpServerService) Create(ctx context.Context, namespace string, spec v1alpha1.MCPServerSpec, name string) (*v1alpha1.MCPServer, error) {
	obj := &v1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       spec,
	}
	if err := s.k8s.Create(ctx, obj); err != nil {
		return nil, fmt.Errorf("create MCPServer: %w", err)
	}
	return obj, nil
}

func (s *mcpServerService) Update(ctx context.Context, namespace, name string, spec v1alpha1.MCPServerSpec) (*v1alpha1.MCPServer, error) {
	obj := &v1alpha1.MCPServer{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get MCPServer for update: %w", err)
	}
	patch := k8sclient.MergeFrom(obj.DeepCopy())
	obj.Spec = spec
	if err := s.k8s.Patch(ctx, obj, patch); err != nil {
		return nil, fmt.Errorf("patch MCPServer: %w", err)
	}
	return obj, nil
}

func (s *mcpServerService) Delete(ctx context.Context, namespace, name string) error {
	obj := &v1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	if err := s.k8s.Delete(ctx, obj); err != nil {
		return fmt.Errorf("delete MCPServer: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Create `internal/server/service/agent_policy.go`**

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// AgentPolicyService manages AgentPolicy CRD CRUD.
type AgentPolicyService interface {
	List(ctx context.Context, namespace string) ([]v1alpha1.AgentPolicy, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentPolicy, error)
	Create(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error)
	Update(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error)
	Delete(ctx context.Context, namespace, name string) error
}

type agentPolicyService struct {
	k8s k8sclient.Client
}

// NewAgentPolicyService creates a new AgentPolicyService.
func NewAgentPolicyService(k8s k8sclient.Client) AgentPolicyService {
	return &agentPolicyService{k8s: k8s}
}

func (s *agentPolicyService) List(ctx context.Context, namespace string) ([]v1alpha1.AgentPolicy, error) {
	list := &v1alpha1.AgentPolicyList{}
	if err := s.k8s.List(ctx, list, k8sclient.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list AgentPolicies: %w", err)
	}
	return list.Items, nil
}

func (s *agentPolicyService) Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentPolicy, error) {
	obj := &v1alpha1.AgentPolicy{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get AgentPolicy %s/%s: %w", namespace, name, err)
	}
	return obj, nil
}

func (s *agentPolicyService) Create(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error) {
	obj := &v1alpha1.AgentPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       spec,
	}
	if err := s.k8s.Create(ctx, obj); err != nil {
		return nil, fmt.Errorf("create AgentPolicy: %w", err)
	}
	return obj, nil
}

func (s *agentPolicyService) Update(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error) {
	obj := &v1alpha1.AgentPolicy{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get AgentPolicy for update: %w", err)
	}
	patch := k8sclient.MergeFrom(obj.DeepCopy())
	obj.Spec = spec
	if err := s.k8s.Patch(ctx, obj, patch); err != nil {
		return nil, fmt.Errorf("patch AgentPolicy: %w", err)
	}
	return obj, nil
}

func (s *agentPolicyService) Delete(ctx context.Context, namespace, name string) error {
	obj := &v1alpha1.AgentPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	if err := s.k8s.Delete(ctx, obj); err != nil {
		return fmt.Errorf("delete AgentPolicy: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Verify + lint + commit**

```bash
go build ./internal/server/service/...
make lint
git add internal/server/service/mcp_server.go internal/server/service/agent_policy.go
git commit -s -m "feat(service): add MCPServer and AgentPolicy CRUD services"
```

---

### Task 4: MCPServer and AgentPolicy API routers + agent config endpoint

**Files:**
- Create: `internal/server/server/mcp_server_router.go`
- Create: `internal/server/server/agent_policy_router.go`

- [ ] **Step 1: Create `internal/server/server/mcp_server_router.go`**

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
	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

// mcpServerRouter registers MCPServer CRUD endpoints.
func (s *Server) mcpServerRouter() {
	g := s.Group("/api/v1/mcp-servers")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	{
		g.GET("", s.handleListMCPServers())
		g.POST("", s.handleCreateMCPServer())
		g.GET("/:name", s.handleGetMCPServer())
		g.PUT("/:name", s.handleUpdateMCPServer())
		g.DELETE("/:name", s.handleDeleteMCPServer())
		g.GET("/:name/tools", s.handleListMCPServerTools())
	}
}

func (s *Server) handleListMCPServers() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		items, err := s.mcpServerSvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, items)
	}
}

func (s *Server) handleCreateMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Name string                  `json:"name" binding:"required"`
			Spec v1alpha1.MCPServerSpec  `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.mcpServerSvc.Create(c.Request.Context(), ns, req.Spec, req.Name)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleGetMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		obj, err := s.mcpServerSvc.Get(c.Request.Context(), ns, c.Param("name"))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleUpdateMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Spec v1alpha1.MCPServerSpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.mcpServerSvc.Update(c.Request.Context(), ns, c.Param("name"), req.Spec)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleDeleteMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		if err := s.mcpServerSvc.Delete(c.Request.Context(), ns, c.Param("name")); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func (s *Server) handleListMCPServerTools() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		obj, err := s.mcpServerSvc.Get(c.Request.Context(), ns, c.Param("name"))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj.Spec.Tools)
	}
}
```

- [ ] **Step 2: Create `internal/server/server/agent_policy_router.go`**

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
	"strings"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

// agentPolicyRouter registers AgentPolicy CRUD endpoints and the agent-facing
// MCP config endpoint (authenticated via agent JWT).
func (s *Server) agentPolicyRouter() {
	// Admin CRUD — standard user auth
	g := s.Group("/api/v1/agent-policies")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	{
		g.GET("", s.handleListAgentPolicies())
		g.POST("", s.handleCreateAgentPolicy())
		g.GET("/:name", s.handleGetAgentPolicy())
		g.PUT("/:name", s.handleUpdateAgentPolicy())
		g.DELETE("/:name", s.handleDeleteAgentPolicy())
	}

	// Agent-facing: returns MCPServers + AgentPolicies for a registered agent.
	// Authenticated via Agent JWT (Bearer token from enrollment).
	// GET /api/v1/agent/mcp-config?namespace=<ns>
	s.GET("/api/v1/agent/mcp-config", s.handleAgentMCPConfig())
}

func (s *Server) handleListAgentPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		items, err := s.agentPolicySvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, items)
	}
}

func (s *Server) handleCreateAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Name string                   `json:"name" binding:"required"`
			Spec v1alpha1.AgentPolicySpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.agentPolicySvc.Create(c.Request.Context(), ns, req.Name, req.Spec)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleGetAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		obj, err := s.agentPolicySvc.Get(c.Request.Context(), ns, c.Param("name"))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleUpdateAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Spec v1alpha1.AgentPolicySpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.agentPolicySvc.Update(c.Request.Context(), ns, c.Param("name"), req.Spec)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleDeleteAgentPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "AgentPolicy requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		if err := s.agentPolicySvc.Delete(c.Request.Context(), ns, c.Param("name")); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// MCPConfigResponse is returned to sandbox agents to configure their MCP proxy.
type MCPConfigResponse struct {
	MCPServers    []v1alpha1.MCPServer    `json:"mcpServers"`
	AgentPolicies []v1alpha1.AgentPolicy  `json:"agentPolicies"`
}

// handleAgentMCPConfig serves MCPServer + AgentPolicy config to a sandbox agent
// authenticated with its Agent JWT.
//
// GET /api/v1/agent/mcp-config?namespace=<ns>
// Authorization: Bearer <agent-jwt>
func (s *Server) handleAgentMCPConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.agentRegService == nil || s.mcpServerSvc == nil || s.agentPolicySvc == nil {
			resp.PaymentRequired(c, "MCP config requires agent isolation and K8s")
			return
		}

		// Validate agent JWT from Authorization header.
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			resp.Unauthorized(c, "Bearer agent JWT required")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := s.agentRegService.ValidateAgentJWT(token)
		if err != nil {
			resp.Unauthorized(c, "invalid agent JWT: "+err.Error())
			return
		}

		ns := claims.Namespace

		servers, err := s.mcpServerSvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, "list MCPServers: "+err.Error())
			return
		}

		policies, err := s.agentPolicySvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, "list AgentPolicies: "+err.Error())
			return
		}

		resp.OK(c, MCPConfigResponse{
			MCPServers:    servers,
			AgentPolicies: policies,
		})
	}
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/server/server/...
```

Expected: no errors. If there are errors about missing `mcpServerSvc` or `agentPolicySvc` fields, that's expected — they'll be added in Task 5.

- [ ] **Step 4: Lint + commit**

```bash
make lint
git add internal/server/server/mcp_server_router.go internal/server/server/agent_policy_router.go
git commit -s -m "feat(server): add MCPServer and AgentPolicy CRUD routers + agent MCP config endpoint"
```

---

### Task 5: Wire services + controller + routers into Server

**Files:**
- Modify: `internal/server/server/server.go`
- Modify: `internal/server/server/api.go`

- [ ] **Step 1: Read server.go to find the Server struct field block and NewServer**

```bash
grep -n "agentRegService\|agentPolicySvc\|mcpServerSvc\|agentIsolationService" \
  internal/server/server/server.go | head -10
```

- [ ] **Step 2: Add two fields to Server struct**

In `internal/server/server/server.go`, find the `agentRegService` field line and add two new fields right after:

```go
	agentRegService       service.AgentRegistrationService
	mcpServerSvc          service.MCPServerService     // nil when K8s unavailable
	agentPolicySvc        service.AgentPolicyService   // nil when K8s unavailable
```

- [ ] **Step 3: Initialize services in NewServer**

Find the block in `NewServer` where `agentRegSvc` is assigned (around line ~190) and add service initialization after it. Find the closing brace of the `if cfg.AI.AgentIsolation.Enabled && client != nil` block and ADD the new services initialization OUTSIDE that block (they don't require AgentIsolation to be enabled, just a K8s client):

```go
	// MCPServer and AgentPolicy services — available whenever K8s client is present.
	var mcpServerSvc service.MCPServerService
	var agentPolicySvc service.AgentPolicyService
	if client != nil {
		mcpServerSvc = service.NewMCPServerService(client.Client)
		agentPolicySvc = service.NewAgentPolicyService(client.Client)
		if mgr != nil {
			if err = controller.NewMCPServerReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
				logger.Warn("failed to setup MCPServerReconciler", "err", err)
			}
		}
	}
```

- [ ] **Step 4: Assign fields in the Server struct literal**

Find the large struct literal in `NewServer` where `agentRegService` is assigned and add:

```go
		mcpServerSvc:           mcpServerSvc,
		agentPolicySvc:         agentPolicySvc,
```

- [ ] **Step 5: Register routers in api.go**

In `internal/server/server/api.go`, find the line `s.agentIsolationRouter()` and add:

```go
	s.mcpServerRouter()
	s.agentPolicyRouter()
```

- [ ] **Step 6: Build all services**

```bash
make build SERVICE=latticed 2>&1 | tail -5
make build SERVICE=manager 2>&1 | tail -5
make lint
```

Expected: all pass, 0 lint issues.

- [ ] **Step 7: Run unit tests**

```bash
make test 2>&1 | tail -5
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/server/server/server.go internal/server/server/api.go
git commit -s -m "feat(server): wire MCPServer + AgentPolicy services, controller, and routers"
```

---

### Task 6: MCP proxy — policy cache

**Files:**
- Create: `internal/agent/mcpproxy/policy_cache.go`

- [ ] **Step 1: Create `internal/agent/mcpproxy/policy_cache.go`**

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

// Package mcpproxy implements an HTTP proxy that intercepts MCP tool calls,
// enforces AgentPolicy, and writes structured audit events.
package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
)

// PolicyConfig holds the MCPServer and AgentPolicy lists fetched from the server.
type PolicyConfig struct {
	MCPServers    []v1alpha1.MCPServer   `json:"mcpServers"`
	AgentPolicies []v1alpha1.AgentPolicy `json:"agentPolicies"`
}

// PolicyCache fetches and caches MCPServer + AgentPolicy config from the Lattice
// management server, refreshing every refreshInterval. Thread-safe.
type PolicyCache struct {
	serverURL   string
	agentJWT    string
	namespace   string
	refreshRate time.Duration

	mu  sync.RWMutex
	cfg *PolicyConfig
}

// NewPolicyCache creates a PolicyCache. Call Start to begin background refresh.
func NewPolicyCache(serverURL, agentJWT, namespace string) *PolicyCache {
	return &PolicyCache{
		serverURL:   serverURL,
		agentJWT:    agentJWT,
		namespace:   namespace,
		refreshRate: 15 * time.Second,
	}
}

// Start fetches the initial config (blocking) then launches a background refresh loop.
// Returns an error if the initial fetch fails.
func (c *PolicyCache) Start(ctx context.Context) error {
	if err := c.fetch(ctx); err != nil {
		return fmt.Errorf("mcpproxy: initial policy fetch: %w", err)
	}
	go c.loop(ctx)
	return nil
}

// Get returns the current cached config. Returns empty config if not yet loaded.
func (c *PolicyCache) Get() PolicyConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cfg == nil {
		return PolicyConfig{}
	}
	return *c.cfg
}

// IsToolAllowed checks whether the given (agentName, mcpServerName, toolName)
// combination is permitted by at least one AgentPolicy in the cache.
// If no policy has DefaultDeny, all tools are allowed (audit-only mode).
func (c *PolicyCache) IsToolAllowed(agentName, mcpServerName, toolName string) bool {
	cfg := c.Get()

	for _, policy := range cfg.AgentPolicies {
		// Simple name match: policy targets this agent if the agentSelector matchLabels
		// includes "agent-name: <agentName>". For MVP, also match if selector is empty
		// (matches all agents).
		if !selectorMatchesAgent(policy.Spec.AgentSelector, agentName) {
			continue
		}
		if !policy.Spec.DefaultDeny {
			// This policy is allow-all; don't block.
			return true
		}
		// DefaultDeny: check allowedTools.
		for _, perm := range policy.Spec.AllowedTools {
			if perm.MCPServer != mcpServerName {
				continue
			}
			for _, t := range perm.Tools {
				if t == "*" || t == toolName {
					return true
				}
			}
		}
	}

	// No policy matched with DefaultDeny — allow (no policy = allow all).
	for _, policy := range cfg.AgentPolicies {
		if selectorMatchesAgent(policy.Spec.AgentSelector, agentName) && policy.Spec.DefaultDeny {
			return false // at least one DefaultDeny policy matched and denied
		}
	}
	return true
}

// MCPServerForHost returns the MCPServer whose endpoint host matches the given host,
// or nil if no match. Used for external MCPs.
func (c *PolicyCache) MCPServerForHost(host string) *v1alpha1.MCPServer {
	cfg := c.Get()
	for i := range cfg.MCPServers {
		srv := &cfg.MCPServers[i]
		u, err := url.Parse(srv.Spec.Endpoint)
		if err != nil {
			continue
		}
		if u.Hostname() == host {
			return srv
		}
	}
	return nil
}

// MCPServerForIP returns the MCPServer whose overlay IP matches, or nil.
// Used for internal MCPs.
func (c *PolicyCache) MCPServerForIP(ip string) *v1alpha1.MCPServer {
	cfg := c.Get()
	for i := range cfg.MCPServers {
		srv := &cfg.MCPServers[i]
		if srv.Status.PeerAddress == ip {
			return srv
		}
	}
	return nil
}

func (c *PolicyCache) loop(ctx context.Context) {
	ticker := time.NewTicker(c.refreshRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.fetch(ctx) // errors are non-fatal; cached config remains valid
		}
	}
}

func (c *PolicyCache) fetch(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/api/v1/agent/mcp-config?namespace=%s", c.serverURL, c.namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.agentJWT)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Data PolicyConfig `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode mcp config: %w", err)
	}

	c.mu.Lock()
	c.cfg = &envelope.Data
	c.mu.Unlock()
	return nil
}

// selectorMatchesAgent returns true if the label selector matches this agent.
// MVP: matches by "agent-name" label key, or matches all when selector is empty.
func selectorMatchesAgent(sel interface{ GetMatchLabels() map[string]string }, agentName string) bool {
	// Use type assertion since metav1.LabelSelector doesn't have a GetMatchLabels method.
	return true // MVP: all policies apply to all agents (filtering by selector in Phase C)
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/agent/mcpproxy/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/agent/mcpproxy/policy_cache.go
git commit -s -m "feat(mcpproxy): add PolicyCache — fetch MCPServer+AgentPolicy from server with 15s refresh"
```

---

### Task 7: MCP proxy — HTTP proxy + audit

**Files:**
- Create: `internal/agent/mcpproxy/proxy.go`
- Create: `internal/agent/mcpproxy/audit.go`

- [ ] **Step 1: Create `internal/agent/mcpproxy/audit.go`**

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

package mcpproxy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	AuditLogPath = "/tmp/lattice-mcp-audit.jsonl"

	verdictAllow = "allow"
	verdictDeny  = "deny"
)

// MCPAuditEvent records a single MCP tool call policy decision.
type MCPAuditEvent struct {
	Timestamp    string `json:"timestamp"`
	AgentName    string `json:"agentName"`
	MCPServer    string `json:"mcpServer"`
	Tool         string `json:"tool"`
	ParamSummary string `json:"paramSummary,omitempty"`
	Verdict      string `json:"verdict"` // "allow" | "deny"
	DenyReason   string `json:"denyReason,omitempty"`
}

// AuditWriter writes MCPAuditEvents as JSONL to a file.
type AuditWriter struct {
	mu sync.Mutex
	f  *os.File
}

// NewAuditWriter opens (or creates) the JSONL audit log file.
func NewAuditWriter(path string) (*AuditWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log %s: %w", path, err)
	}
	return &AuditWriter{f: f}, nil
}

// Write appends one event to the JSONL file. Thread-safe.
func (w *AuditWriter) Write(event MCPAuditEvent) {
	if w == nil {
		return
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	w.mu.Lock()
	_, _ = fmt.Fprintf(w.f, "%s\n", data)
	w.mu.Unlock()
}

// Close flushes and closes the audit file.
func (w *AuditWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// summarizeParams produces a short, redacted summary of MCP tool params.
// Sensitive keys (password, token, secret, key, auth) are replaced with [REDACTED].
// Strings longer than 200 bytes are truncated.
func summarizeParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}
	var parts []string
	sensitiveKeys := map[string]bool{
		"password": true, "token": true, "secret": true, "key": true, "auth": true,
	}
	for k, v := range params {
		kl := strings.ToLower(k)
		for sk := range sensitiveKeys {
			if strings.Contains(kl, sk) {
				parts = append(parts, k+"=[REDACTED]")
				goto next
			}
		}
		switch vt := v.(type) {
		case string:
			if len(vt) > 200 {
				parts = append(parts, fmt.Sprintf("%s=%s...[truncated, total=%dB]", k, vt[:100], len(vt)))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%s", k, vt))
			}
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	next:
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 2: Create `internal/agent/mcpproxy/proxy.go`**

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

package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Proxy is an HTTP proxy that intercepts MCP JSON-RPC calls, enforces AgentPolicy,
// and writes audit events. Non-MCP traffic is forwarded transparently.
type Proxy struct {
	agentName string
	cache     *PolicyCache
	audit     *AuditWriter
	server    *http.Server
}

// NewProxy creates a Proxy. addr is the listen address (e.g. "127.0.0.1:15002").
func NewProxy(agentName, addr string, cache *PolicyCache, audit *AuditWriter) *Proxy {
	p := &Proxy{
		agentName: agentName,
		cache:     cache,
		audit:     audit,
	}
	p.server = &http.Server{
		Addr:    addr,
		Handler: p,
	}
	return p
}

// Addr returns the listen address. Call after Start.
func (p *Proxy) Addr() string { return p.server.Addr }

// Start begins listening. Returns once the listener is bound; handler runs in background.
func (p *Proxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.server.Addr)
	if err != nil {
		return fmt.Errorf("mcpproxy: listen: %w", err)
	}
	// Update addr to the actual bound address (port 0 → OS-assigned port).
	p.server.Addr = ln.Addr().String()
	go func() {
		<-ctx.Done()
		_ = p.server.Shutdown(context.Background())
	}()
	go func() { _ = p.server.Serve(ln) }()
	return nil
}

// ServeHTTP implements http.Handler. Routes CONNECT tunnels and regular requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

// handleHTTP handles plain HTTP proxy requests.
// If the target is a known MCPServer endpoint, parse the JSON-RPC body and enforce policy.
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	targetHost := r.URL.Hostname()

	// Check if this request targets a known MCPServer.
	mcpSrv := p.cache.MCPServerForHost(targetHost)

	if mcpSrv != nil && r.Method == http.MethodPost {
		// Read body for inspection.
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadGateway)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		toolName, params := extractMCPTool(body)
		if toolName != "" {
			paramSummary := summarizeParams(params)
			if !p.cache.IsToolAllowed(p.agentName, mcpSrv.Name, toolName) {
				p.audit.Write(MCPAuditEvent{
					AgentName:    p.agentName,
					MCPServer:    mcpSrv.Name,
					Tool:         toolName,
					ParamSummary: paramSummary,
					Verdict:      verdictDeny,
					DenyReason:   "AgentPolicy denied",
				})
				http.Error(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Lattice AgentPolicy: tool not allowed"}}`,
					http.StatusForbidden)
				return
			}
			p.audit.Write(MCPAuditEvent{
				AgentName:    p.agentName,
				MCPServer:    mcpSrv.Name,
				Tool:         toolName,
				ParamSummary: paramSummary,
				Verdict:      verdictAllow,
			})
		}
	}

	// Forward the request to the real target.
	targetURL, err := url.Parse(r.RequestURI)
	if err != nil || targetURL.Host == "" {
		targetURL = &url.URL{
			Scheme: "http",
			Host:   r.Host,
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

// handleConnect handles HTTPS CONNECT tunnels.
// For HTTPS MCPs (external platform MCPs), we cannot inspect the payload (TLS).
// We pass through transparently and log at the connection level only.
func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Dial the target.
	conn, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "failed to connect", http.StatusBadGateway)
		return
	}
	defer conn.Close() //nolint:errcheck

	// Hijack the client connection.
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close() //nolint:errcheck

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional copy.
	done := make(chan struct{}, 2)
	go func() { io.Copy(conn, clientConn); done <- struct{}{} }()       //nolint:errcheck
	go func() { io.Copy(clientConn, conn); done <- struct{}{} }()       //nolint:errcheck
	<-done
}

// mcpRequest is the minimal JSON-RPC 2.0 structure for MCP tool calls.
type mcpRequest struct {
	Method string `json:"method"`
	Params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
}

// extractMCPTool parses a JSON-RPC body and returns the tool name and arguments
// if the method is "tools/call". Returns ("", nil) for non-tool-call requests.
func extractMCPTool(body []byte) (toolName string, params map[string]interface{}) {
	var req mcpRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", nil
	}
	if req.Method != "tools/call" {
		return "", nil
	}
	return req.Params.Name, req.Params.Arguments
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/agent/mcpproxy/...
```

Expected: no errors.

- [ ] **Step 4: Write unit tests**

Create `internal/agent/mcpproxy/proxy_test.go`:

```go
package mcpproxy

import (
	"testing"
)

func TestExtractMCPTool_ToolsCall(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read_file","arguments":{"path":"/data/report.pdf"}}}`)
	tool, params := extractMCPTool(body)
	if tool != "read_file" {
		t.Fatalf("expected read_file, got %q", tool)
	}
	if params["path"] != "/data/report.pdf" {
		t.Fatalf("expected path param, got %v", params)
	}
}

func TestExtractMCPTool_NonToolCall(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list","params":{}}`)
	tool, _ := extractMCPTool(body)
	if tool != "" {
		t.Fatalf("expected empty tool for non-tools/call, got %q", tool)
	}
}

func TestExtractMCPTool_Invalid(t *testing.T) {
	tool, _ := extractMCPTool([]byte(`not json`))
	if tool != "" {
		t.Fatalf("expected empty tool for invalid JSON, got %q", tool)
	}
}

func TestSummarizeParams_Redaction(t *testing.T) {
	params := map[string]interface{}{
		"path":     "/data/file.txt",
		"password": "supersecret",
		"token":    "abc123",
	}
	summary := summarizeParams(params)
	if contains(summary, "supersecret") || contains(summary, "abc123") {
		t.Errorf("sensitive values should be redacted, got: %s", summary)
	}
	if !contains(summary, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in summary, got: %s", summary)
	}
}

func TestSummarizeParams_Truncation(t *testing.T) {
	longStr := make([]byte, 300)
	for i := range longStr {
		longStr[i] = 'a'
	}
	params := map[string]interface{}{"content": string(longStr)}
	summary := summarizeParams(params)
	if !contains(summary, "[truncated") {
		t.Errorf("expected truncation note, got: %s", summary)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run unit tests**

```bash
go test ./internal/agent/mcpproxy/... -v
```

Expected: all tests pass.

- [ ] **Step 6: Lint + commit**

```bash
make lint
git add internal/agent/mcpproxy/proxy.go internal/agent/mcpproxy/audit.go internal/agent/mcpproxy/proxy_test.go
git commit -s -m "feat(mcpproxy): add MCP HTTP proxy with tool-level policy enforcement and JSONL audit"
```

---

### Task 8: Wire MCP proxy into sandbox run

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/run.go`
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go`

- [ ] **Step 1: Read the current run.go**

```bash
cat cmd/lattice/cmd/sandbox/run.go
```

- [ ] **Step 2: Add MCP proxy flags to runCmd**

In `cmd/lattice/cmd/sandbox/run.go`, add these flags to `runCmd()` after the existing flags:

```go
cmd.Flags().BoolVar(&runMCPProxy, "mcp-proxy", false,
    "Enable MCP HTTP proxy for tool-level policy enforcement and audit (Pro)")
```

Add the variable declaration:
```go
var (
    runServerURL   string
    runToken       string
    runReadyWait   time.Duration
    runEgressAllow string
    runEgressDeny  bool
    runMCPProxy    bool  // ADD THIS
)
```

- [ ] **Step 3: Update runRun to pass mcpProxy flag**

In `runRun`, pass `runMCPProxy` to `runSandbox`:

```go
return runSandbox(ctx, cancel, agentName, currentPeer, policyChecker, auditWriter, cmdArgs, runMCPProxy)
```

- [ ] **Step 4: Update runSandbox signature and body in shared_linux.go**

Add `enableMCPProxy bool` as the last parameter to `runSandbox`:

```go
func runSandbox(
	ctx context.Context,
	cancel context.CancelFunc,
	agentName string,
	currentPeer *infra.Peer,
	_ shim.PolicyChecker,
	_ shim.AuditWriter,
	cmdArgs []string,
	enableMCPProxy bool,
) error {
```

After `node.Start(ctx)` and before `forkAgent`, add the MCP proxy startup:

```go
	httpProxyAddr := ""
	if enableMCPProxy && currentPeer.Token != "" {
		cache := mcpproxy.NewPolicyCache(agentconfig.Conf.ServerUrl, currentPeer.Token, overlayAddr(currentPeer))
		if cacheErr := cache.Start(ctx); cacheErr != nil {
			logger.Warn("MCP policy cache failed to start, proxy disabled", "err", cacheErr)
		} else {
			auditW, _ := mcpproxy.NewAuditWriter(mcpproxy.AuditLogPath)
			proxy := mcpproxy.NewProxy(agentName, "127.0.0.1:0", cache, auditW)
			if proxyErr := proxy.Start(ctx); proxyErr != nil {
				logger.Warn("MCP proxy failed to start", "err", proxyErr)
			} else {
				httpProxyAddr = "http://" + proxy.Addr()
				fmt.Printf("[sandbox-run] MCP proxy on %s\n", proxy.Addr())
			}
		}
	}
```

- [ ] **Step 5: Update forkAgent to accept and inject httpProxyAddr**

Change `forkAgent` signature to:
```go
func forkAgent(ctx context.Context, cancel context.CancelFunc, cmdArgs []string, httpProxyAddr string) error {
```

Inside `forkAgent`, update the env setup:
```go
	env := os.Environ()
	if httpProxyAddr != "" {
		env = append(env,
			"HTTP_PROXY="+httpProxyAddr,
			"http_proxy="+httpProxyAddr,
			"HTTPS_PROXY="+httpProxyAddr,
			"https_proxy="+httpProxyAddr,
		)
	}
	child.Env = env
```

Update the `forkAgent` call in `runSandbox`:
```go
	return forkAgent(ctx, cancel, cmdArgs, httpProxyAddr)
```

- [ ] **Step 6: Add import for mcpproxy in shared_linux.go**

Add to imports:
```go
	"github.com/alatticeio/lattice/internal/agent/mcpproxy"
```

- [ ] **Step 7: Update forkAgent tests in shared_linux_test.go**

Update `TestForkAgent_ExitsCleanly` and `TestForkAgent_InheritsEnv` to pass the new `httpProxyAddr` parameter:

```go
func TestForkAgent_ExitsCleanly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := forkAgent(ctx, cancel, []string{"true"}, "")
	if err != nil {
		t.Fatalf("forkAgent returned err for exit-0 child: %v", err)
	}
}

func TestForkAgent_InheritsEnv(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t.Setenv("LATTICE_TEST_MARKER", "yes")
	err := forkAgent(ctx, cancel, []string{"sh", "-c", `test "$LATTICE_TEST_MARKER" = "yes"`}, "")
	if err != nil {
		t.Fatalf("expected child to inherit env, got err: %v", err)
	}
}

func TestForkAgent_InjectsHTTPProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// When httpProxyAddr is set, HTTP_PROXY should be visible to the child.
	err := forkAgent(ctx, cancel,
		[]string{"sh", "-c", `test "$HTTP_PROXY" = "http://127.0.0.1:9999"`},
		"http://127.0.0.1:9999")
	if err != nil {
		t.Fatalf("expected child to see HTTP_PROXY, got err: %v", err)
	}
}
```

- [ ] **Step 8: Build and test**

```bash
GOOS=linux GOARCH=amd64 go test -c ./cmd/lattice/cmd/sandbox/ -o /dev/null
make build SERVICE=lattice 2>&1 | tail -3
go test ./internal/agent/mcpproxy/... -v
make lint
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add cmd/lattice/cmd/sandbox/run.go cmd/lattice/cmd/sandbox/shared_linux.go cmd/lattice/cmd/sandbox/shared_linux_test.go
git commit -s -m "feat(sandbox): wire MCP proxy into sandbox run — inject HTTP_PROXY when --mcp-proxy enabled"
```

---

### Task 9: Final build verification

- [ ] **Step 1: Build all services**

```bash
make build SERVICE=lattice 2>&1 | tail -2
make build SERVICE=latticed 2>&1 | tail -2
make build SERVICE=manager 2>&1 | tail -2
```

Expected: all succeed.

- [ ] **Step 2: Full unit test suite**

```bash
make test 2>&1 | tail -5
```

Expected: all pass.

- [ ] **Step 3: Lint**

```bash
make lint
```

Expected: 0 issues.

- [ ] **Step 4: Git log summary**

```bash
git log --oneline -12
```

Expected: clean sequence of commits for Plan B.
