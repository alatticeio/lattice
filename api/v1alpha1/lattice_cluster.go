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

// ClusterPhase is the lifecycle phase of a LatticeCluster connection.
type ClusterPhase string

const (
	ClusterPhaseConnected    ClusterPhase = "Connected"
	ClusterPhaseDisconnected ClusterPhase = "Disconnected"
	ClusterPhaseUnknown      ClusterPhase = "Unknown"
)

// LatticeClusterSpec describes a remote Lattice deployment that this cluster
// can establish cross-cluster peerings with.
type LatticeClusterSpec struct {
	// ManagementEndpoint is the HTTPS base URL of the remote cluster's
	// Lattice management API (e.g. "https://lattice.prod-eu.example.com").
	ManagementEndpoint string `json:"managementEndpoint"`

	// CredentialRef is the name of a Secret in the controller namespace that
	// holds the authentication token for the remote management API.
	// The Secret must have key "token".
	CredentialRef string `json:"credentialRef"`
}

// LatticeClusterStatus reports the observed connection state of a remote cluster.
type LatticeClusterStatus struct {
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

// LatticeCluster registers a remote Lattice deployment so that
// LatticeClusterPeering resources can reference it.
type LatticeCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeClusterSpec   `json:"spec,omitempty"`
	Status            LatticeClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LatticeClusterList contains a list of LatticeCluster.
type LatticeClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticeCluster `json:"items"`
}

// ---------------------------------------------------------------------------
// LatticeClusterPeering
// ---------------------------------------------------------------------------

// LatticeClusterPeeringSpec declares a cross-cluster network peering.
type LatticeClusterPeeringSpec struct {
	// LocalNamespace is the workspace namespace in this cluster.
	LocalNamespace string `json:"localNamespace"`
	// LocalNetwork is the LatticeNetwork name in LocalNamespace.
	LocalNetwork string `json:"localNetwork"`

	// RemoteCluster references a LatticeCluster resource that describes the
	// remote Lattice deployment.
	RemoteCluster string `json:"remoteCluster"`
	// RemoteNamespace is the workspace namespace in the remote cluster.
	RemoteNamespace string `json:"remoteNamespace"`
	// RemoteNetwork is the LatticeNetwork name in RemoteNamespace.
	RemoteNetwork string `json:"remoteNetwork"`
}

// ClusterPeeringPhase is the lifecycle phase of a LatticeClusterPeering.
type ClusterPeeringPhase string

const (
	ClusterPeeringPhasePending ClusterPeeringPhase = "Pending"
	ClusterPeeringPhaseReady   ClusterPeeringPhase = "Ready"
	ClusterPeeringPhaseError   ClusterPeeringPhase = "Error"
)

// LatticeClusterPeeringStatus reports the observed state of a cross-cluster peering.
type LatticeClusterPeeringStatus struct {
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

// LatticeClusterPeering connects a local workspace network with a network in a
// remote Lattice cluster via the gateway-mode peering mechanism.
type LatticeClusterPeering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeClusterPeeringSpec   `json:"spec,omitempty"`
	Status            LatticeClusterPeeringStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LatticeClusterPeeringList contains a list of LatticeClusterPeering.
type LatticeClusterPeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticeClusterPeering `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LatticeCluster{}, &LatticeClusterList{})
	SchemeBuilder.Register(&LatticeClusterPeering{}, &LatticeClusterPeeringList{})
}
