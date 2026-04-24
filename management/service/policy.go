package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wireflow/api/v1alpha1"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/resource"
	"wireflow/management/vo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PolicyService interface {
	// Submit saves the policy to DB with status=pending and returns the record.
	// Called when a non-admin user creates a policy (workflow approval required).
	Submit(ctx context.Context, wsID, createdBy, createdByName string, policyDto *dto.PolicyDto) (*models.Policy, error)

	// Apply writes the policy to k8s and marks the DB record as active.
	// Called by the workflow executor after approval, or directly by admin on PUT.
	Apply(ctx context.Context, policyID string) error

	// ApplyDirect writes to k8s immediately and upserts a DB record with status=active.
	// Used for admin direct-update (PUT /update).
	ApplyDirect(ctx context.Context, wsID string, policyDto *dto.PolicyDto) (*vo.PolicyVo, error)

	ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error)
	DeletePolicy(ctx context.Context, name string) error
}

type policyService struct {
	log    *log.Logger
	client *resource.Client
	store  store.Store
}

func NewPolicyService(client *resource.Client, st store.Store) PolicyService {
	return &policyService{
		log:    log.GetLogger("policy-service"),
		client: client,
		store:  st,
	}
}

// Submit stores the policy intent in DB as "pending" (awaiting workflow approval).
func (p *policyService) Submit(ctx context.Context, wsID, createdBy, createdByName string, policyDto *dto.PolicyDto) (*models.Policy, error) {
	specBytes, err := json.Marshal(policyDto.WireflowPolicySpec)
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}
	typesBytes, err := json.Marshal(policyDto.PolicyTypes)
	if err != nil {
		return nil, fmt.Errorf("marshal policy types: %w", err)
	}

	rec := &models.Policy{
		WorkspaceID:   wsID,
		Name:          policyDto.Name,
		Description:   policyDto.Description,
		Action:        policyDto.Action,
		PolicyTypes:   string(typesBytes),
		Spec:          string(specBytes),
		Status:        models.PolicyStatusPending,
		CreatedBy:     createdBy,
		CreatedByName: createdByName,
	}
	if err := p.store.Policies().Create(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Apply is called by the workflow executor. It reads the DB record, writes to k8s,
// and updates DB status to active (or failed on error).
func (p *policyService) Apply(ctx context.Context, policyID string) error {
	rec, err := p.store.Policies().GetByID(ctx, policyID)
	if err != nil {
		return fmt.Errorf("policy record not found: %w", err)
	}

	workspace, err := p.store.Workspaces().GetByID(ctx, rec.WorkspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	var spec v1alpha1.WireflowPolicySpec
	if err := json.Unmarshal([]byte(rec.Spec), &spec); err != nil {
		return fmt.Errorf("unmarshal spec: %w", err)
	}
	var policyTypes []string
	_ = json.Unmarshal([]byte(rec.PolicyTypes), &policyTypes)

	spec.Action = rec.Action

	crd := &v1alpha1.WireflowPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "wireflowcontroller.wireflow.run/v1alpha1",
			Kind:       "WireflowPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rec.Name,
			Namespace: workspace.Namespace,
			Labels:    map[string]string{"action": rec.Action},
			Annotations: map[string]string{
				"description": rec.Description,
				"policyTypes": strings.Join(policyTypes, ","),
			},
		},
		Spec: spec,
	}

	manager := client.FieldOwner("wireflow-controller-manager")
	if err := p.client.Patch(ctx, crd, client.Apply, manager); err != nil {
		rec.Status = models.PolicyStatusFailed
		rec.ErrorMessage = err.Error()
		_ = p.store.Policies().Update(ctx, rec)
		return err
	}

	rec.Status = models.PolicyStatusActive
	rec.ErrorMessage = ""
	return p.store.Policies().Update(ctx, rec)
}

// ApplyDirect is used by platform_admin PUT /update — writes directly to k8s and
// upserts the DB record as active.
func (p *policyService) ApplyDirect(ctx context.Context, wsID string, policyDto *dto.PolicyDto) (*vo.PolicyVo, error) {
	workspace, err := p.store.Workspaces().GetByID(ctx, wsID)
	if err != nil {
		return nil, err
	}

	spec := policyDto.WireflowPolicySpec
	spec.Action = policyDto.Action

	crd := &v1alpha1.WireflowPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "wireflowcontroller.wireflow.run/v1alpha1",
			Kind:       "WireflowPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyDto.Name,
			Namespace: workspace.Namespace,
			Labels:    map[string]string{"action": policyDto.Action},
			Annotations: map[string]string{
				"description": policyDto.Description,
				"policyTypes": strings.Join(policyDto.PolicyTypes, ","),
			},
		},
		Spec: spec,
	}

	manager := client.FieldOwner("wireflow-controller-manager")
	if err := p.client.Patch(ctx, crd, client.Apply, manager); err != nil {
		return nil, err
	}

	// Upsert DB record.
	specBytes, _ := json.Marshal(policyDto.WireflowPolicySpec)
	typesBytes, _ := json.Marshal(policyDto.PolicyTypes)

	existing, err := p.store.Policies().GetByName(ctx, wsID, policyDto.Name)
	if err != nil {
		// Create new.
		existing = &models.Policy{
			WorkspaceID: wsID,
			Name:        policyDto.Name,
		}
	}
	existing.Description = policyDto.Description
	existing.Action = policyDto.Action
	existing.PolicyTypes = string(typesBytes)
	existing.Spec = string(specBytes)
	existing.Status = models.PolicyStatusActive
	existing.ErrorMessage = ""

	if existing.ID == "" {
		_ = p.store.Policies().Create(ctx, existing)
	} else {
		_ = p.store.Policies().Update(ctx, existing)
	}

	return &vo.PolicyVo{
		Name:               policyDto.Name,
		Action:             policyDto.Action,
		Description:        policyDto.Description,
		Namespace:          policyDto.Namespace,
		PolicyTypes:        policyDto.PolicyTypes,
		WireflowPolicySpec: &spec,
	}, nil
}

// ListPolicy reads from DB — the single source of truth for all policy states.
func (p *policyService) ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error) {
	wsID, _ := ctx.Value(infra.WorkspaceKey).(string)

	records, total, err := p.store.Policies().List(ctx, store.PolicyFilter{
		WorkspaceID: wsID,
		Keyword:     pageParam.Keyword,
		Page:        pageParam.Page,
		PageSize:    pageParam.PageSize,
	})
	if err != nil {
		return nil, err
	}

	vos := make([]vo.PolicyVo, 0, len(records))
	for _, rec := range records {
		var spec v1alpha1.WireflowPolicySpec
		_ = json.Unmarshal([]byte(rec.Spec), &spec)
		var policyTypes []string
		_ = json.Unmarshal([]byte(rec.PolicyTypes), &policyTypes)

		vos = append(vos, vo.PolicyVo{
			Name:               rec.Name,
			Action:             rec.Action,
			Description:        rec.Description,
			PolicyTypes:        policyTypes,
			Status:             string(rec.Status),
			CreatedByName:      rec.CreatedByName,
			WireflowPolicySpec: &spec,
		})
	}

	return &dto.PageResult[vo.PolicyVo]{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Total:    total,
		List:     vos,
	}, nil
}

// DeletePolicy removes the CRD from k8s, the DB record, and any associated workflow request.
func (p *policyService) DeletePolicy(ctx context.Context, name string) error {
	wsID, _ := ctx.Value(infra.WorkspaceKey).(string)

	workspace, err := p.store.Workspaces().GetByID(ctx, wsID)
	if err != nil {
		return err
	}

	// Fetch the policy record before deleting so we can clean up its workflow request.
	rec, _ := p.store.Policies().GetByName(ctx, wsID, name)

	crd := &v1alpha1.WireflowPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WireflowPolicy",
			APIVersion: "wireflowcontroller.wireflow.run/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: workspace.Namespace,
		},
	}
	// Best-effort CRD deletion (may not exist if policy is still pending).
	_ = p.client.Delete(ctx, crd)

	if err := p.store.Policies().Delete(ctx, wsID, name); err != nil {
		return err
	}

	// Clean up the associated workflow request if one exists.
	if rec != nil && rec.WorkflowRequestID != "" {
		_ = p.store.WorkflowRequests().Delete(ctx, rec.WorkflowRequestID)
	}

	return nil
}
