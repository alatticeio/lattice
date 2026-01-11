package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true

type WireflowEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireflowEndpointSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

type WireflowEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowEndpoint `json:"items"`
}

type WireflowEndpointSpec struct {
	Address    string `json:"address"`
	PeerRef    string `json:"peerRef"`
	NetworkRef string `json:"networkRef"`
}

func init() {
	SchemeBuilder.Register(&WireflowEndpoint{}, &WireflowEndpointList{})
}
