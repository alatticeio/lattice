package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true

// +kubebuilder:resource:shortName=wfe
// +kubebuilder:printcolumn:name="ADDRESS",type="string",JSONPath=".status.address",description="The current address of the peer"
// +kubebuilder:printcolumn:name="PEER-REF",type="string",JSONPath=".spec.peerref",description="the endpoint's peer reference"
type LatticeEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeEndpointSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

type LatticeEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LatticeEndpoint `json:"items"`
}

type LatticeEndpointSpec struct {
	Address    string `json:"address"`
	PeerRef    string `json:"peerRef"`
	NetworkRef string `json:"networkRef"`
}

func init() {
	SchemeBuilder.Register(&LatticeEndpoint{}, &LatticeEndpointList{})
}
