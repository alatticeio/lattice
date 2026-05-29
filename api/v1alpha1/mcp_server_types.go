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
// +kubebuilder:validation:Enum=low;medium;high;critical
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// MCPServerPhase is the lifecycle phase of an MCPServer.
// +kubebuilder:validation:Enum=Pending;Ready;Degraded
type MCPServerPhase string

const (
	MCPServerPhasePending  MCPServerPhase = "Pending"
	MCPServerPhaseReady    MCPServerPhase = "Ready"
	MCPServerPhaseDegraded MCPServerPhase = "Degraded"
)

// MCPServerMode is the connectivity mode of an MCPServer.
// +kubebuilder:validation:Enum=internal;external
type MCPServerMode string

const (
	MCPServerModeInternal MCPServerMode = "internal"
	MCPServerModeExternal MCPServerMode = "external"
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
	Mode MCPServerMode `json:"mode,omitempty"`
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
