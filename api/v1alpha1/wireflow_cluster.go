// Copyright 2025 The Wireflow Authors, Inc.
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

// ClusterPhase is the lifecycle phase of a WireflowCluster connection.
type ClusterPhase string

const (
	ClusterPhaseConnected    ClusterPhase = "Connected"
	ClusterPhaseDisconnected ClusterPhase = "Disconnected"
	ClusterPhaseUnknown      ClusterPhase = "Unknown"
)

// WireflowClusterSpec describes a remote Wireflow deployment that this cluster
// can establish cross-cluster peerings with.
type WireflowClusterSpec struct {
	// ManagementEndpoint is the HTTPS base URL of the remote cluster's
	// Wireflow management API (e.g. "https://wireflow.prod-eu.example.com").
	ManagementEndpoint string `json:"managementEndpoint"`

	// CredentialRef is the name of a Secret in the controller namespace that
	// holds the authentication token for the remote management API.
	// The Secret must have key "token".
	CredentialRef string `json:"credentialRef"`
}

// WireflowClusterStatus reports the observed connection state of a remote cluster.
type WireflowClusterStatus struct {
	// Phase is the current connection state: Connected | Disconnected | Unknown.
	Phase ClusterPhase `json:"phase,omitempty"`

	// LastProbeTime is when the controller last successfully contacted the remote cluster.
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`

	// Conditions contains fine-grained status conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=wfcluster
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".spec.managementEndpoint"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// WireflowCluster registers a remote Wireflow deployment so that
// WireflowClusterPeering resources can reference it.
type WireflowCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireflowClusterSpec   `json:"spec,omitempty"`
	Status            WireflowClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WireflowClusterList contains a list of WireflowCluster.
type WireflowClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowCluster `json:"items"`
}

// ---------------------------------------------------------------------------
// WireflowClusterPeering
// ---------------------------------------------------------------------------

// WireflowClusterPeeringSpec declares a cross-cluster network peering.
type WireflowClusterPeeringSpec struct {
	// LocalNamespace is the workspace namespace in this cluster.
	LocalNamespace string `json:"localNamespace"`
	// LocalNetwork is the WireflowNetwork name in LocalNamespace.
	LocalNetwork string `json:"localNetwork"`

	// RemoteCluster references a WireflowCluster resource that describes the
	// remote Wireflow deployment.
	RemoteCluster string `json:"remoteCluster"`
	// RemoteNamespace is the workspace namespace in the remote cluster.
	RemoteNamespace string `json:"remoteNamespace"`
	// RemoteNetwork is the WireflowNetwork name in RemoteNamespace.
	RemoteNetwork string `json:"remoteNetwork"`
}

// ClusterPeeringPhase is the lifecycle phase of a WireflowClusterPeering.
type ClusterPeeringPhase string

const (
	ClusterPeeringPhasePending ClusterPeeringPhase = "Pending"
	ClusterPeeringPhaseReady   ClusterPeeringPhase = "Ready"
	ClusterPeeringPhaseError   ClusterPeeringPhase = "Error"
)

// WireflowClusterPeeringStatus reports the observed state of a cross-cluster peering.
type WireflowClusterPeeringStatus struct {
	// Phase is Pending | Ready | Error.
	Phase ClusterPeeringPhase `json:"phase,omitempty"`

	// LocalCIDR is the ActiveCIDR of the local network, populated once Ready.
	LocalCIDR string `json:"localCIDR,omitempty"`

	// RemoteCIDR is the ActiveCIDR of the remote network, populated once Ready.
	RemoteCIDR string `json:"remoteCIDR,omitempty"`

	// Conditions contains fine-grained status conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=wfcpeering
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="LOCAL-CIDR",type="string",JSONPath=".status.localCIDR"
// +kubebuilder:printcolumn:name="REMOTE-CIDR",type="string",JSONPath=".status.remoteCIDR"
// +kubebuilder:printcolumn:name="REMOTE-CLUSTER",type="string",JSONPath=".spec.remoteCluster"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// WireflowClusterPeering connects a local workspace network with a network in a
// remote Wireflow cluster via the gateway-mode peering mechanism.
type WireflowClusterPeering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireflowClusterPeeringSpec   `json:"spec,omitempty"`
	Status            WireflowClusterPeeringStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WireflowClusterPeeringList contains a list of WireflowClusterPeering.
type WireflowClusterPeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowClusterPeering `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WireflowCluster{}, &WireflowClusterList{})
	SchemeBuilder.Register(&WireflowClusterPeering{}, &WireflowClusterPeeringList{})
}
