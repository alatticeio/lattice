package dto

import "github.com/alatticeio/lattice/api/v1alpha1"

type PolicyDto struct {
	Name        string   `json:"name" binding:"required,min=1,max=64"`       // Must be lowercase English only
	Action      string   `json:"action" binding:"required,oneof=Allow Deny"` // Allow / Deny
	Description string   `json:"description"`
	PolicyTypes []string `json:"policyTypes" binding:"required"` // e.g. ["Ingress","Egress"]
	// Namespace is intentionally omitted: the server derives it from the workspace context.
	// Network is provided via the embedded LatticePolicySpec.Network field below.
	v1alpha1.LatticePolicySpec
}
