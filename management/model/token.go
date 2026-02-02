package model

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Token struct {
	Token      string      `json:"token"`
	Namespace  string      `json:"namespace"`
	UsageLimit int         `json:"usageLimit"`
	Expiry     metav1.Time `json:"expiry"`
	BoundPeers []string    `json:"boundPeers,omitempty"`
}
