package vo

import (
	"wireflow/api/v1alpha1"
)

type PolicyVo struct {
	Name          string   `json:"name"`
	Action        string   `json:"action"`
	Description   string   `json:"description"`
	Namespace     string   `json:"namespace"`
	PolicyTypes   []string `json:"policyTypes"`
	// Status reflects the DB record status: pending / approved / active / failed
	Status        string   `json:"status,omitempty"`
	CreatedByName string   `json:"createdByName,omitempty"`
	*v1alpha1.WireflowPolicySpec `json:",inline"`
}
