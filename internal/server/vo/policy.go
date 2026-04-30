package vo

import (
	"github.com/alatticeio/lattice/api/v1alpha1"
)

type PolicyVo struct {
	Name        string   `json:"name"`
	Action      string   `json:"action"`
	Description string   `json:"description"`
	Namespace   string   `json:"namespace"`
	PolicyTypes []string `json:"policyTypes"`
	// Status reflects the DB record status: pending / approved / active / failed
	Status                      string `json:"status,omitempty"`
	CreatedBy                   string `json:"createdBy,omitempty"`
	CreatedByName               string `json:"createdByName,omitempty"`
	CreatedAt                   string `json:"createdAt,omitempty"`
	UpdatedBy                   string `json:"updatedBy,omitempty"`
	UpdatedByName               string `json:"updatedByName,omitempty"`
	UpdatedAt                   string `json:"updatedAt,omitempty"`
	*v1alpha1.LatticePolicySpec `json:",inline"`
}
