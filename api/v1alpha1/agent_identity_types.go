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
	SandboxNone    SandboxMode = "none"    // not hosted by Lattice
	SandboxPod     SandboxMode = "pod"     // K8s Pod + seccomp (community)
	SandboxGVisor  SandboxMode = "gvisor"  // gVisor runsc user-space kernel (Pro)
	SandboxMicroVM SandboxMode = "microvm" // Firecracker MicroVM (Pro, future)
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
	AgentConditionPeerBound = "PeerBound"
	AgentConditionJWTIssued = "JWTIssued"
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
