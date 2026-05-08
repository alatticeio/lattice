package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="TOTAL",type="integer",JSONPath=".status.totalSubnets"
// +kubebuilder:printcolumn:name="ALLOCATED",type="integer",JSONPath=".status.allocatedSubnets"
// +kubebuilder:printcolumn:name="AVAILABLE",type="integer",JSONPath=".status.availableSubnets"
// +kubebuilder:printcolumn:name="USAGE",type="string",JSONPath=".status.usagePercentage"
// +kubebuilder:resource:scope=Cluster,shortName=wfpool
type LatticeGlobalIPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeGlobalIPPoolSpec   `json:"spec"`
	Status            LatticeGlobalIPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// +kubebuilder:resource:scope=Cluster
type LatticeGlobalIPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticeGlobalIPPool `json:"items"`
}

// LatticeGlobalIPPoolSpec define global ip pool
type LatticeGlobalIPPoolSpec struct {
	CIDR       string `json:"cidr"`       // e.g. "10.0.0.0/8"
	SubnetMask int    `json:"subnetMask"` // How much each Network gets, e.g. 24
}

// LatticeGlobalIPPoolStatus =
type LatticeGlobalIPPoolStatus struct {
	// TotalSubnets total subnets
	TotalSubnets int `json:"totalSubnets"`

	// AllocatedSubnets
	AllocatedSubnets int `json:"allocatedSubnets"`

	// AvailableSubnets
	AvailableSubnets int `json:"availableSubnets"`

	// UsagePercentage
	UsagePercentage string `json:"usagePercentage"`

	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=wfsubnet

// LatticeSubnetAllocation for store / search a network's cidr
// Its Name format is: subnet-<hex-ip> (e.g. subnet-0a0a0100)
// +kubebuilder:resource:scope=Cluster,shortName=wfsubnet
type LatticeSubnetAllocation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeSubnetAllocationSpec `json:"spec"`
}

// +kubebuilder:object:root=true

type LatticeSubnetAllocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticeSubnetAllocation `json:"items"`
}

type LatticeSubnetAllocationSpec struct {
	NetworkName string `json:"networkName"` // Which Network it belongs to
	CIDR        string `json:"cidr"`        // Actual allocated prefix, e.g. 10.10.1.0/24
}

func init() {
	SchemeBuilder.Register(&LatticeGlobalIPPool{}, &LatticeGlobalIPPoolList{})
	SchemeBuilder.Register(&LatticeSubnetAllocation{}, &LatticeSubnetAllocationList{})
}
