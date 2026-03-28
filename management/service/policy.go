package service

import (
	"context"
	"strings"
	"wireflow/api/v1alpha1"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/management/vo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PolicyService interface {
	CreateOrUpdatePolicy(ctx context.Context, policyDto *dto.PolicyDto) (*vo.PolicyVo, error)
	ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error)
	DeletePolicy(ctx context.Context, name string) error
}

type policyService struct {
	log    *log.Logger
	client *resource.Client
	store  store.Store
}

func (p *policyService) DeletePolicy(ctx context.Context, name string) error {
	wsId := ctx.Value(infra.WorkspaceKey).(string)
	workspace, err := p.store.Workspaces().GetByID(ctx, wsId)
	if err != nil {
		return err
	}

	res := &v1alpha1.WireflowPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WireflowPolicy",
			APIVersion: "wireflowcontroller.wireflow.run/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: workspace.Namespace,
		},
	}
	return p.client.Delete(ctx, res)
}

func (p *policyService) ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error) {
	workspaceV := ctx.Value(infra.WorkspaceKey)
	var workspaceId string
	if workspaceV != nil {
		workspaceId = workspaceV.(string)
	}

	workspace, err := p.store.Workspaces().GetByID(ctx, workspaceId)
	if err != nil {
		return nil, err
	}

	var policyList v1alpha1.WireflowPolicyList
	if err = p.client.GetAPIReader().List(ctx, &policyList, client.InNamespace(workspace.Namespace)); err != nil {
		return nil, err
	}

	allPolicies := []*vo.PolicyVo{}

	for _, n := range policyList.Items {
		allPolicies = append(allPolicies, &vo.PolicyVo{
			Name:               n.Name,
			Action:             n.Annotations["action"],
			Description:        n.Annotations["description"],
			WireflowPolicySpec: &n.Spec,
		})
	}

	var filteredPolicies []*vo.PolicyVo
	if pageParam.Keyword != "" {
		for _, n := range allPolicies {
			if strings.Contains(n.Name, pageParam.Keyword) || strings.Contains(n.Action, pageParam.Keyword) || strings.Contains(n.Description, pageParam.Keyword) {
				filteredPolicies = append(filteredPolicies, n)
			}
		}
	} else {
		filteredPolicies = allPolicies
	}

	total := len(filteredPolicies)
	start := (pageParam.Page - 1) * pageParam.PageSize
	end := start + pageParam.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var vos []vo.PolicyVo
	for _, n := range filteredPolicies[start:end] {
		vos = append(vos, *n)
	}

	return &dto.PageResult[vo.PolicyVo]{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Total:    int64(len(allPolicies)),
		List:     vos,
	}, nil
}

func (p *policyService) CreateOrUpdatePolicy(ctx context.Context, policyDto *dto.PolicyDto) (*vo.PolicyVo, error) {
	wsId := ctx.Value(infra.WorkspaceKey).(string)
	workspace, err := p.store.Workspaces().GetByID(ctx, wsId)
	if err != nil {
		return nil, err
	}

	newPolicy := &v1alpha1.WireflowPolicy{
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
			},
		},
		Spec: policyDto.WireflowPolicySpec,
	}

	manager := client.FieldOwner("wireflow-controller-manager")
	if err = p.client.Patch(ctx, newPolicy, client.Apply, manager); err != nil {
		return nil, err
	}

	return &vo.PolicyVo{
		Name:               newPolicy.Name,
		Action:             newPolicy.Spec.Action,
		Description:        policyDto.Description,
		Namespace:          policyDto.Namespace,
		WireflowPolicySpec: &newPolicy.Spec,
	}, nil
}

func NewPolicyService(client *resource.Client, st store.Store) PolicyService {
	return &policyService{
		log:    log.GetLogger("policy-service"),
		client: client,
		store:  st,
	}
}
