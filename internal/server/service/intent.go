package service

import (
	"context"
	"fmt"
	"time"
)

// CRDChange describes a single K8s resource change.
type CRDChange struct {
	Action   string `json:"action"`   // create | update | delete
	Resource string `json:"resource"` // e.g. "policy/allow-frontend-to-api"
	Before   string `json:"before"`   // YAML of current state (empty if new)
	After    string `json:"after"`    // YAML of desired state (empty if deleting)
}

// IntentRequest is the input for Plan.
type IntentRequest struct {
	WorkspaceID string
	Namespace   string
	Intent      string // natural language
	DryRun      bool
}

// IntentPlanView is returned to the caller (not the DB model).
type IntentPlanView struct {
	ID        string      `json:"id"`
	Summary   string      `json:"summary"` // Markdown
	Changes   []CRDChange `json:"changes"`
	RiskLevel string      `json:"riskLevel"` // low | medium | high
	ExpiresAt time.Time   `json:"expiresAt"`
}

// IntentHistoryItem is a single applied plan summary for the history list.
type IntentHistoryItem struct {
	ID        string     `json:"id"`
	Intent    string     `json:"intent"`
	Summary   string     `json:"summary"`
	RiskLevel string     `json:"riskLevel"`
	AppliedAt *time.Time `json:"appliedAt"`
	AppliedBy string     `json:"appliedBy"`
}

// IntentService translates natural language intent into CRD change plans.
type IntentService interface {
	Plan(ctx context.Context, req IntentRequest) (*IntentPlanView, error)
	Apply(ctx context.Context, planID, approvedBy string) ([]string, error) // returns workflowIDs
	History(ctx context.Context, workspaceID string, limit int) ([]*IntentHistoryItem, error)
}

// ErrPaymentRequired returns a sentinel error for Pro-only features.
// The HTTP layer converts this to a 402 response.
func ErrPaymentRequired(feature string) error {
	return fmt.Errorf("%s (upgrade to Pro: https://alattice.io/pro)", feature)
}
