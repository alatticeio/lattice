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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LatticePeerSpec defines the desired state of LatticePeer.
type LatticePeerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	AppId string `json:"appId,omitempty"`

	// Interface for the node
	InterfaceName string `json:"interfaceName,omitempty"`

	// platform which node runs on
	Platform string `json:"platform,omitempty"`

	PrivateKey string `json:"privateKey,omitempty"`

	PublicKey string `json:"publicKey,omitempty"`

	AllowedIPs []string `json:"allowedIPs,omitempty"`

	DNSServers []string `json:"dnsServers,omitempty"`

	MTU int `json:"mtu,omitempty"`

	PeerId string `json:"peerId,omitempty"`

	Network *string `json:"network,omitempty"`

	NetworkPolicies []string `json:"networkPolicies,omitempty"`

	// WrrpUrl is the TCP address of the WRRP relay server assigned to this peer.
	// Populated by the relay settings controller when a relay is bound to the peer's workspace.
	WrrpUrl string `json:"wrrpUrl,omitempty"`

	// WrrpQuicUrl is the QUIC address of the WRRP relay server.
	// When set, nodes prefer QUIC over TCP for relay traffic.
	WrrpQuicUrl string `json:"wrrpQuicUrl,omitempty"`
}

// LatticePeerStatus defines the observed state of LatticePeer.
type LatticePeerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LatticePeer status
	Status Status `json:"status,omitempty"`

	Phase LatticePeerPhase `json:"phase,omitempty"`

	// Active key
	ActiveKey string `json:"activeKey,omitempty"`

	// Active networks, record the network the node joined
	ActiveNetwork *string `json:"activeNetwork,omitempty"`

	ActiveNetworkPolicies []string `json:"activeNetworkPolicies,omitempty"`

	// Allocated IP address, auto allocated by controller
	AllocatedAddress *string `json:"allocatedAddress,omitempty"`

	// Connection summary
	ConnectionSummary ConnectionSummary `json:"connectionSummary,omitempty"`

	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// message hash store here
	CurrentHash string `json:"currentHash,omitempty"`

	// client applied version
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type Status string

const (
	Active   Status = "Active"
	InActive Status = "inactive"
	Stopped  Status = "stopped"
)

type LatticePeerPhase string

const (
	NodePhasePending      LatticePeerPhase = "Pending"
	NodePhaseProvisioning LatticePeerPhase = "Provisioning"
	NodePhaseFailed       LatticePeerPhase = "Failed"
	NodePhaseReady        LatticePeerPhase = "Ready"
)

// Condition Types
const (
	NodeConditionInitialized = "Initialized"

	// NodeConditionProvisioned 节点是否就绪
	NodeConditionProvisioned = "Provisioned"

	NodeConditionJoiningNetwork = "JoiningNetwork"

	// NodeConditionNetworkConfigured 网络配置是否完成
	NodeConditionNetworkConfigured = "NetworkConfigured"

	// NodeConditionIPAllocated IP 是否已分配
	NodeConditionIPAllocated = "IPAllocated"

	NodeConditionPolicyUpdating = "PolicyUpdating"

	// NodeConditionPolicyApplied 策略是否已应用
	NodeConditionPolicyApplied = "PolicyApplied"
)

// Condition Reasons
const (
	ReasonInitializing     = "Initializing"
	ReasonAllocating       = "Allocating"
	ReasonConfiguring      = "Configuring"
	ReasonReady            = "Ready"
	ReasonNotReady         = "NotReady"
	ReasonUpdating         = "Updating"
	ReasonLeaving          = "Leaving"
	ReasonAllocationFailed = "AllocationFailed"
	ReasonConfigFailed     = "ConfigurationFailed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LatticePeer is the Schema for the nodes API.
// +kubebuilder:resource:shortName=wfpeer
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.status",description="The current status of the node"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="The current phase of the node"
// +kubebuilder:printcolumn:name="IP",type="string",JSONPath=".status.allocatedAddress",description="The IP address allocated to the node"
// +kubebuilder:printcolumn:name="NETWORK",type="string",JSONPath=".spec.network",description="The network the node belongs to"
// +kubebuilder:printcolumn:name="CONNECTED",type="integer",JSONPath=".status.connectionSummary.connected",description="Number of active connections"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type LatticePeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LatticePeerSpec   `json:"spec,omitempty"`
	Status LatticePeerStatus `json:"status,omitempty"`
}

// ConnectionSummary represents connection summary
type ConnectionSummary struct {
	Total        int `json:"total"`
	Connected    int `json:"connected"`
	Disconnected int `json:"disconnected"`
}

// +kubebuilder:object:root=true

// LatticePeerList contains a list of LatticePeer.
type LatticePeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticePeer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LatticePeer{}, &LatticePeerList{})
}
