package vo

import (
	"wireflow/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicyVo struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Description  string                 `json:"description"`
	PeerSelector metav1.LabelSelector   `json:"peerSelector"`
	IngressRule  []v1alpha1.IngressRule `json:"ingressRule,omitempty"`
	EgressRule   []v1alpha1.EgressRule  `json:"egressRule,omitempty"`
	Network      string                 `json:"network"`
	Action       string                 `json:"action"`
}
