package dto

import "github.com/alatticeio/lattice/api/v1alpha1"

type PolicyDto struct {
	Name        string   `json:"name" binding:"required,min=1,max=64"` // Must be lowercase English only
	Namespace   string   `json:"namespace" binding:"required,min=1,max=64"`
	Action      string   `json:"action" binding:"required,oneof=Allow Deny"` // Allow / Deny
	Description string   `json:"description"`
	PolicyTypes []string `json:"policyTypes" binding:"required"` // e.g. ["Ingress","Egress"]
	Network     string   `json:"network" binding:"required"`
	v1alpha1.LatticePolicySpec
}
