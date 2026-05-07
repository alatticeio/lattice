//go:build pro

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/llm"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/resource"

	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type intentService struct {
	logger *log.Logger
	llm    llm.Client
	k8s    *resource.Client
	store  store.Store
}

// NewIntentService creates a Pro-edition IntentService with real LLM-driven intent processing.
func NewIntentService(lm llm.Client, k8s *resource.Client, st store.Store) IntentService {
	return &intentService{
		logger: log.GetLogger("intent"),
		llm:    lm,
		k8s:    k8s,
		store:  st,
	}
}

// NewIntentServiceForTest creates an IntentService for unit tests, bypassing the Pro build tag.
func NewIntentServiceForTest(lm llm.Client, k8s *resource.Client, st store.Store) IntentService {
	return NewIntentService(lm, k8s, st)
}

func (s *intentService) Plan(ctx context.Context, req IntentRequest) (*IntentPlanView, error) {
	// Snapshot current K8s state for LLM context.
	stateContext, err := s.buildStateContext(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("build state context: %w", err)
	}

	// Phase 1: structured change extraction.
	changes, riskLevel, err := s.extractChanges(ctx, req.Intent, stateContext)
	if err != nil {
		return nil, fmt.Errorf("extract changes: %w", err)
	}

	// Phase 2: human-readable diff generation.
	summary, err := s.generateSummary(ctx, changes)
	if err != nil {
		s.logger.Warn("summary generation failed, using minimal summary", "err", err)
		summary = fmt.Sprintf("将执行 %d 项变更。", len(changes))
	}

	changesJSON, _ := json.Marshal(changes)
	plan := &models.IntentPlan{
		ID:          uuid.New().String(),
		WorkspaceID: req.WorkspaceID,
		Namespace:   req.Namespace,
		Intent:      req.Intent,
		Summary:     summary,
		ChangesJSON: string(changesJSON),
		RiskLevel:   riskLevel,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	if !req.DryRun {
		if err := s.store.IntentPlans().Create(ctx, plan); err != nil {
			return nil, fmt.Errorf("save plan: %w", err)
		}
	}

	return &IntentPlanView{
		ID:        plan.ID,
		Summary:   summary,
		Changes:   changes,
		RiskLevel: riskLevel,
		ExpiresAt: plan.ExpiresAt,
	}, nil
}

func (s *intentService) Apply(ctx context.Context, planID, approvedBy string) ([]string, error) {
	plan, err := s.store.IntentPlans().GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}
	if time.Now().After(plan.ExpiresAt) {
		return nil, fmt.Errorf("plan %q has expired — please re-run Plan to get a fresh plan", planID)
	}

	var changes []CRDChange
	if err := json.Unmarshal([]byte(plan.ChangesJSON), &changes); err != nil {
		return nil, fmt.Errorf("parse changes: %w", err)
	}

	// Return plan ID — the HTTP handler converts changes to workflow requests.
	return []string{planID}, nil
}

// buildStateContext returns a compact text representation of current K8s state.
func (s *intentService) buildStateContext(ctx context.Context, namespace string) (string, error) {
	var peers v1alpha1.LatticePeerList
	if err := s.k8s.List(ctx, &peers, client.InNamespace(namespace)); err != nil {
		// Non-fatal: proceed with partial context
		s.logger.Warn("failed to list peers for intent context", "err", err)
	}

	var policies v1alpha1.LatticePolicyList
	if err := s.k8s.List(ctx, &policies, client.InNamespace(namespace)); err != nil {
		s.logger.Warn("failed to list policies for intent context", "err", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 当前状态（命名空间: %s）\n\n", namespace))
	sb.WriteString(fmt.Sprintf("### Peers (%d)\n", len(peers.Items)))
	for _, p := range peers.Items {
		sb.WriteString(fmt.Sprintf("- %s labels=%v\n", p.Name, p.Labels))
	}
	sb.WriteString(fmt.Sprintf("\n### Policies (%d)\n", len(policies.Items)))
	for _, p := range policies.Items {
		sb.WriteString(fmt.Sprintf("- %s [%s] network=%s\n", p.Name, p.Spec.Action, p.Spec.Network))
	}
	return sb.String(), nil
}

// extractChanges calls the LLM for phase-1 structured extraction.
func (s *intentService) extractChanges(ctx context.Context, intent, stateContext string) ([]CRDChange, string, error) {
	prompt := fmt.Sprintf(`You are a Lattice network policy expert. Based on the user intent and current network state, generate the list of CRD changes needed.

%s

## User Intent
%s

## Output Requirements
Return a JSON object with the following format (return ONLY the JSON, no other content):
{
  "changes": [
    {
      "action": "create|update|delete",
      "resource": "policy/<name>",
      "before": "<current YAML, empty for new resources>",
      "after": "<desired YAML, empty for deletion>"
    }
  ],
  "riskLevel": "low|medium|high",
  "reasoning": "<brief explanation>"
}`, stateContext, intent)

	resp, err := s.llm.Complete(ctx, &llm.Request{
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 2048,
	})
	if err != nil {
		return nil, "", err
	}

	content := strings.TrimSpace(resp.Content)
	if idx := strings.Index(content, "{"); idx >= 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "}"); idx >= 0 {
		content = content[:idx+1]
	}

	var result struct {
		Changes   []CRDChange `json:"changes"`
		RiskLevel string      `json:"riskLevel"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, "", fmt.Errorf("parse LLM response: %w\nraw: %s", err, content)
	}
	if result.RiskLevel == "" {
		result.RiskLevel = "medium"
	}
	return result.Changes, result.RiskLevel, nil
}

// generateSummary calls the LLM for phase-2 human-readable diff.
func (s *intentService) generateSummary(ctx context.Context, changes []CRDChange) (string, error) {
	changesJSON, _ := json.MarshalIndent(changes, "", "  ")
	prompt := fmt.Sprintf(`The following is a Lattice network policy change plan (JSON format):
%s

Generate a concise Markdown summary explaining:
1. What changes will happen (one line per change)
2. Expected effect
3. Potential risks (if any)

Return only the Markdown content, no code block wrapping.`, string(changesJSON))

	resp, err := s.llm.Complete(ctx, &llm.Request{
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
