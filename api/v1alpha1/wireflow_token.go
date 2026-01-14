package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// +kubebuilder:resource:shortName=wftoken
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",JSONPath=".spec.namespace",description="invite to namespace"
// +kubebuilder:printcolumn:name="USAGELIMIT",type="string",JSONPath=".spec.usagelimit",description="limit to invite"
type WireflowEnrollmentToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireflowEnrollmentTokenSpec   `json:"spec,omitempty"`
	Status            WireflowEnrollmentTokenStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type WireflowEnrollmentTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowEnrollmentToken `json:"items"`
}

type WireflowEnrollmentTokenSpec struct {
	Token      string      `json:"token"`
	Namespace  string      `json:"namespace"`
	UsageLimit int         `json:"usageLimit"`
	Expiry     metav1.Time `json:"expiry"`
	BoundPeers []string    `json:"boundPeers,omitempty"`
}

type WireflowEnrollmentTokenStatus struct {
	Token      string   `json:"token,omitempty"`
	BoundPeers []string `json:"boundPeers,omitempty"`
	Phase      string   `json:"phase,omitempty"` // Acitve / Expired / Exhausted
	UsedCount  int      `json:"usedCount,omitempty"`
	IsExpired  bool     `json:"isExpired,omitempty"`
}

func init() {
	SchemeBuilder.Register(&WireflowEnrollmentToken{}, &WireflowEnrollmentTokenList{})
}
