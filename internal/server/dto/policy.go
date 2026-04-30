package dto

import "github.com/alatticeio/lattice/api/v1alpha1"

type PolicyDto struct {
	Name        string   `json:"name"` // 只能是小写英文
	Namespace   string   `json:"namespace"`
	Action      string   `json:"action"` // Allow / Deny
	Description string   `json:"description"`
	PolicyTypes []string `json:"policyTypes"` // e.g. ["Ingress","Egress"]
	v1alpha1.LatticePolicySpec
}
