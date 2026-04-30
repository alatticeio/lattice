package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LatticeInvitation is the Schema for the networks API.
// +kubebuilder:resource:shortName=wfinv
// +kubebuilder:printcolumn:name="NETWORK",type="string",JSONPath=".spec.network",description="Joined Network"
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",JSONPath=".spec.namespace",description="Joined namespace"
// +kubebuilder:printcolumn:name="LABELS",type="string",JSONPath=".spec.peerLabels",description="Added labels when joined"
// +kubebuilder:printcolumn:name="ExpiresAt",type="string",JSONPath=".spec.expiresAt",description="Expires time"
type LatticeInvitation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LatticeInvitationSpec   `json:"spec"`
	Status            LatticeInvitationStatus `json:"status,omitempty"`
}

type LatticeInvitationSpec struct {
	// Invite a peer join to a network
	Network string `json:"network"`

	// join to special namespace
	TargetNamespace string `json:"targetNamespace"`

	// 预设给新 Peer 的标签（例如 role: developer）
	PeerLabels map[string]string `json:"peerLabels,omitempty"`

	ExpiresAt metav1.Time `json:"expiresAt"`
}

type LatticeInvitationStatus struct {
	// 生成给客户端的token
	Token string `json:"token"`

	// 已使用的次数
	UsedCount int `json:"usedCount"`

	UsedPeers []string `json:"usedPeers"`

	ExpiresAt metav1.Time `json:"expiresAt"`
}

// +kubebuilder:object:root=true

type LatticeInvitationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
}

func init() {
	SchemeBuilder.Register(&LatticeInvitation{}, &LatticeInvitationList{})
}
