# Remove k8s Dependency from server/dto/policy.go

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove `k8s.io/apimachinery` (~3MB, 2471 symbols) from the `lattice` agent binary by decoupling `server/dto/policy.go` from `api/v1alpha1`.

**Architecture:** `server/dto/policy.go` currently embeds `v1alpha1.LatticePolicySpec` which pulls in `k8s.io/apimachinery`. We define a plain-Go `PolicySpec` in `server/dto/policy_types.go` (no k8s imports), have `PolicyDto` embed it instead, and use JSON roundtrip to convert to `v1alpha1.LatticePolicySpec` only at the server-side k8s CRD write boundary. JSON format is fully compatible between the plain types and k8s types.

**Tech Stack:** Go 1.25, standard `encoding/json`

**Branch:** `feat/reduce-binary-size`

---

## File Map

| Action | File |
|--------|------|
| Create | `internal/server/dto/policy_types.go` |
| Modify | `internal/server/dto/policy.go` |
| Modify | `internal/server/service/policy.go` |
| Modify | `internal/server/server/demo.go` |

**NOT changing:** `internal/server/vo/policy.go` — `server/vo` is no longer in the agent dep graph (fixed in previous work), so this file doesn't affect agent binary size.

---

## Task 1: Create internal/server/dto/policy_types.go

Plain-Go mirrors of `v1alpha1.LatticePolicySpec` and related types. Zero k8s imports.

**Files:**
- Create: `internal/server/dto/policy_types.go`

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

package dto

import "time"

// PolicySpec mirrors v1alpha1.LatticePolicySpec using plain Go types.
// JSON field names are identical so DB records and wire format remain compatible.
type PolicySpec struct {
	Network      string        `json:"network,omitempty"`
	PeerSelector LabelSelector `json:"peerSelector,omitempty"`
	Ingress      []IngressRule `json:"ingress,omitempty"`
	Egress       []EgressRule  `json:"egress,omitempty"`
	Action       string        `json:"action,omitempty"`
	ExpiresAt    *time.Time    `json:"expiresAt,omitempty"`
}

// LabelSelector mirrors metav1.LabelSelector.
type LabelSelector struct {
	MatchLabels      map[string]string          `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement mirrors metav1.LabelSelectorRequirement.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// IngressRule mirrors v1alpha1.IngressRule.
type IngressRule struct {
	From  []PeerSelection     `json:"from,omitempty"`
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// EgressRule mirrors v1alpha1.EgressRule.
type EgressRule struct {
	To    []PeerSelection     `json:"to,omitempty"`
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// PeerSelection mirrors v1alpha1.PeerSelection.
type PeerSelection struct {
	PeerSelector *LabelSelector `json:"peerSelector,omitempty"`
	IPBlock      *IPBlock       `json:"ipBlock,omitempty"`
}

// IPBlock mirrors v1alpha1.IPBlock.
type IPBlock struct {
	CIDR string `json:"cidr,omitempty"`
}

// NetworkPolicyPort mirrors v1alpha1.NetworkPolicyPort.
type NetworkPolicyPort struct {
	Port     int32  `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./internal/server/dto/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/dto/policy_types.go
git commit -s -m "refactor(dto): add plain-Go PolicySpec types to decouple from k8s"
```

---

## Task 2: Update server/dto/policy.go

Replace the embedded `v1alpha1.LatticePolicySpec` with `PolicySpec`.

**Files:**
- Modify: `internal/server/dto/policy.go`

- [ ] **Step 1: Rewrite the file**

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

package dto

type PolicyDto struct {
	Name        string   `json:"name" binding:"required,min=1,max=64"`
	Action      string   `json:"action" binding:"required,oneof=Allow Deny"`
	Description string   `json:"description"`
	PolicyTypes []string `json:"policyTypes" binding:"required"`
	PolicySpec
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: compile errors in `service/policy.go` and `server/demo.go` — these are expected and will be fixed in the next tasks.

- [ ] **Step 3: Verify dto package no longer imports v1alpha1**

```bash
grep -n "v1alpha1" internal/server/dto/policy.go internal/server/dto/policy_types.go
```
Expected: no output.

- [ ] **Step 4: Commit (even with build errors in other packages — they get fixed next)**

Actually, do NOT commit yet. Wait until Tasks 3 and 4 fix all callers. Only commit once `go build ./...` passes cleanly.

---

## Task 3: Update service/policy.go

Fix the 4 locations that reference `policyDto.LatticePolicySpec`.

**Files:**
- Modify: `internal/server/service/policy.go`

**Context:** `service/policy.go` can still import `v1alpha1` — it's server-side. The JSON of `dto.PolicySpec` and `v1alpha1.LatticePolicySpec` are identical, so roundtrip is safe.

- [ ] **Step 1: Fix Submit() — line ~55**

Find:
```go
specBytes, err := json.Marshal(policyDto.LatticePolicySpec)
```
Replace with:
```go
specBytes, err := json.Marshal(policyDto.PolicySpec)
```

- [ ] **Step 2: Fix ApplyDirect() — the spec construction for k8s CRD (~line 141)**

Find:
```go
spec := policyDto.LatticePolicySpec
spec.Action = policyDto.Action
if spec.Network == "" {
    spec.Network = "lattice-default-net"
}

crd := &v1alpha1.LatticePolicy{
    ...
    Spec: spec,
}
```

Replace with (convert plain PolicySpec → v1alpha1.LatticePolicySpec via JSON roundtrip):
```go
// Convert plain dto.PolicySpec to v1alpha1.LatticePolicySpec for k8s write.
var spec v1alpha1.LatticePolicySpec
if specJSON, merr := json.Marshal(policyDto.PolicySpec); merr == nil {
    _ = json.Unmarshal(specJSON, &spec)
}
spec.Action = policyDto.Action
if spec.Network == "" {
    spec.Network = "lattice-default-net"
}

crd := &v1alpha1.LatticePolicy{
    ...
    Spec: spec,
}
```

- [ ] **Step 3: Fix ApplyDirect() — the DB upsert spec marshal (~line 170)**

Find:
```go
specBytes, _ := json.Marshal(policyDto.LatticePolicySpec)
```
Replace with:
```go
specBytes, _ := json.Marshal(policyDto.PolicySpec)
```

- [ ] **Step 4: Fix ApplyDirect() — building the PolicyVo return (~line 198)**

Find:
```go
return &vo.PolicyVo{
    Name:              policyDto.Name,
    Action:            policyDto.Action,
    Description:       policyDto.Description,
    Namespace:         workspace.Namespace,
    PolicyTypes:       policyDto.PolicyTypes,
    LatticePolicySpec: &spec,
}, nil
```

`vo.PolicyVo` still uses `*v1alpha1.LatticePolicySpec` (we're not changing vo). So this line stays as-is — `spec` is already `v1alpha1.LatticePolicySpec` from Step 2. No change needed here.

- [ ] **Step 5: Fix ListPolicy() — the spec unmarshal for PolicyVo (~line 224)**

Find:
```go
var spec v1alpha1.LatticePolicySpec
_ = json.Unmarshal([]byte(rec.Spec), &spec)
...
vos = append(vos, vo.PolicyVo{
    ...
    LatticePolicySpec: &spec,
})
```

This reads from DB and builds PolicyVo. `vo.PolicyVo` still uses `*v1alpha1.LatticePolicySpec`, so this is unchanged — leave as-is.

- [ ] **Step 6: Build**

```bash
go build ./...
```
Expected: only `server/demo.go` still has errors (fixed in Task 4).

---

## Task 4: Update server/demo.go

Replace `v1alpha1.LatticePolicySpec{...}` and `metav1.LabelSelector{...}` with `dto.PolicySpec{...}` and `dto.LabelSelector{...}` in the two PolicyDto constructions.

**Files:**
- Modify: `internal/server/server/demo.go`

- [ ] **Step 1: Fix first PolicyDto construction (~line 166)**

Find:
```go
peerSel := metav1.LabelSelector{
    MatchLabels: map[string]string{networkLabel: "true"},
}
if _, policyErr := s.policyController.ApplyDirect(tokenCtx, wsVo.ID, "", "", &dto.PolicyDto{
    Name:        "demo-allow-all",
    Action:      "Allow",
    PolicyTypes: []string{"Ingress", "Egress"},
    LatticePolicySpec: v1alpha1.LatticePolicySpec{
        Network:      demoNetwork,
        PeerSelector: peerSel,
        Action:       "ALLOW",
        Ingress: []v1alpha1.IngressRule{
            {From: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
        },
        Egress: []v1alpha1.EgressRule{
            {To: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
        },
    },
}); policyErr != nil {
```

Replace with:
```go
peerSel := dto.LabelSelector{
    MatchLabels: map[string]string{networkLabel: "true"},
}
if _, policyErr := s.policyController.ApplyDirect(tokenCtx, wsVo.ID, "", "", &dto.PolicyDto{
    Name:        "demo-allow-all",
    Action:      "Allow",
    PolicyTypes: []string{"Ingress", "Egress"},
    PolicySpec: dto.PolicySpec{
        Network:      demoNetwork,
        PeerSelector: peerSel,
        Action:       "ALLOW",
        Ingress: []dto.IngressRule{
            {From: []dto.PeerSelection{{PeerSelector: &peerSel}}},
        },
        Egress: []dto.EgressRule{
            {To: []dto.PeerSelection{{PeerSelector: &peerSel}}},
        },
    },
}); policyErr != nil {
```

- [ ] **Step 2: Fix second PolicyDto construction (~line 348)**

Same pattern — find and replace the second occurrence (same structure, around line 348-366).

- [ ] **Step 3: Remove now-unused imports from demo.go**

After the changes, check if `v1alpha1` and `metav1` imports are still needed in demo.go:
```bash
grep -n "v1alpha1\.\|metav1\." internal/server/server/demo.go
```

If no remaining uses, remove those imports.

- [ ] **Step 4: Build everything**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Run tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 6: Lint**

```bash
make lint
```
Expected: 0 issues.

- [ ] **Step 7: Commit all three changed files together**

```bash
git add internal/server/dto/policy.go internal/server/service/policy.go internal/server/server/demo.go
git commit -s -m "refactor(dto): decouple PolicyDto from v1alpha1 to remove k8s from agent binary"
```

---

## Verification

After all tasks:

```bash
# 1. api/v1alpha1 should be gone from agent deps
go list -deps ./cmd/lattice/ | grep "api/v1alpha1\|k8s\.io/apimachinery"
# Expected: no output

# 2. Binary size
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/lattice-final ./cmd/lattice/
ls -lh /tmp/lattice-final
# Expected: ~26MB (down from 29MB)

# 3. All tests pass
make test

# 4. Lint clean
make lint
```
