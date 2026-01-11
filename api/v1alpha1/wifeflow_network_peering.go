package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Using for peering network, two networks will be connected together when they have same ip and different namespace
type WireflowNetworkPeering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireflowEndpointSpec `json:"spec,omitempty"`
}

type WireflowNetworkPeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowNetworkPeering `json:"items"`
}

type WireflowNetworkPeeringSpec struct {
	// 租户 A 的网络
	NamespaceA string `json:"namespaceA"`
	NetworkA   string `json:"networkA"`

	// 租户 B 的网络
	NamespaceB string `json:"namespaceB"`
	NetworkB   string `json:"networkB"`

	// 通讯策略：是全局 VIP 模式，还是影子网段模式？
	PeeringMode string `json:"peeringMode"`
}

type WireflowNetworkPeeringStatus struct {
	Phase WireflowNetworkPhase `json:"phase,omitempty"`
}
