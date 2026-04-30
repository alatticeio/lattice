package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PeeringMode controls how traffic is forwarded between peered networks.
// +kubebuilder:validation:Enum=gateway;mesh
type PeeringMode string

const (
	// PeeringModeGateway routes cross-workspace traffic through a designated
	// gateway peer in each workspace. This is the default and recommended mode.
	PeeringModeGateway PeeringMode = "gateway"

	// PeeringModeMesh establishes direct WireGuard tunnels between every peer in
	// each workspace. Suitable only for small deployments.
	PeeringModeMesh PeeringMode = "mesh"
)

// LatticeNetworkPeeringSpec declares a peering relationship between two networks
// that may reside in different namespaces (workspaces).
type LatticeNetworkPeeringSpec struct {
	// NamespaceA is the Kubernetes namespace of the first workspace.
	NamespaceA string `json:"namespaceA"`
	// NetworkA is the LatticeNetwork name in NamespaceA.
	NetworkA string `json:"networkA"`

	// NamespaceB is the Kubernetes namespace of the second workspace.
	NamespaceB string `json:"namespaceB"`
	// NetworkB is the LatticeNetwork name in NamespaceB.
	NetworkB string `json:"networkB"`

	// PeeringMode controls the traffic forwarding strategy.
	// Defaults to "gateway".
	// +kubebuilder:default=gateway
	PeeringMode PeeringMode `json:"peeringMode,omitempty"`
}

// LatticeNetworkPeeringStatus reports the observed state of the peering.
type LatticeNetworkPeeringStatus struct {
	// Phase summarises the peering lifecycle: Pending | Ready | Error.
	Phase LatticeNetworkPhase `json:"phase,omitempty"`

	// CIDRA is the ActiveCIDR of NetworkA, populated once the peering is Ready.
	CIDRA string `json:"cidrA,omitempty"`

	// CIDRB is the ActiveCIDR of NetworkB, populated once the peering is Ready.
	CIDRB string `json:"cidrB,omitempty"`

	// Conditions contains fine-grained status conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=wfpeering
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="CIDR-A",type="string",JSONPath=".status.cidrA"
// +kubebuilder:printcolumn:name="CIDR-B",type="string",JSONPath=".status.cidrB"
// +kubebuilder:printcolumn:name="MODE",type="string",JSONPath=".spec.peeringMode"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// LatticeNetworkPeering connects two LatticeNetworks across different workspaces
// so their peers can communicate directly over WireGuard.
type LatticeNetworkPeering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeNetworkPeeringSpec   `json:"spec,omitempty"`
	Status            LatticeNetworkPeeringStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LatticeNetworkPeeringList contains a list of LatticeNetworkPeering.
type LatticeNetworkPeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticeNetworkPeering `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LatticeNetworkPeering{}, &LatticeNetworkPeeringList{})
}
