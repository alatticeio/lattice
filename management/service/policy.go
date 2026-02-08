package service

import (
	"context"
	"strings"
	"wireflow/api/v1alpha1"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/management/vo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PolicyService interface {
	CreatePolicy(ctx context.Context, namespace, name, action string, labels map[string]string, ingressRules []v1alpha1.IngressRule, egressRules []v1alpha1.EgressRule) error
	ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error)
	UpdatePolicy(ctx context.Context, policyDto *dto.PeerDto) (*vo.PolicyVo, error)
}

type policyService struct {
	log    *log.Logger
	client *resource.Client
}

func (p policyService) ListPolicy(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PolicyVo], error) {
	var (
		policyList v1alpha1.WireflowPolicyList
		err        error
	)
	err = p.client.GetAPIReader().List(ctx, &policyList, client.InNamespace(pageParam.Namespace))

	if err != nil {
		return nil, err
	}

	// 2. 获取全量数据（模拟）
	allPolicies := []*vo.PolicyVo{ /* ... 很多数据 ... */ }

	for _, n := range policyList.Items {
		allPolicies = append(allPolicies, &vo.PolicyVo{
			Name:         n.Name,
			Type:         n.Annotations["type"],
			Description:  n.Annotations["description"],
			PeerSelector: n.Spec.PeerSelector,
			IngressRule:  n.Spec.IngressRule,
			EgressRule:   n.Spec.EgressRule,
			Network:      n.Spec.Network,
			Action:       n.Spec.Action,
		})
	}

	// 3. 逻辑过滤（搜索）
	var filteredPolicies []*vo.PolicyVo
	if pageParam.Search != "" {
		for _, n := range allPolicies {

			policyType := n.Type
			description := n.Description

			if strings.Contains(n.Name, pageParam.Search) || strings.Contains(policyType, pageParam.Search) || strings.Contains(description, pageParam.Search) {
				filteredPolicies = append(filteredPolicies, n)
			}
		}
	} else {
		filteredPolicies = allPolicies
	}

	// 4. 执行内存切片分页
	total := len(filteredPolicies)
	start := (pageParam.Page - 1) * pageParam.PageSize
	end := start + pageParam.PageSize

	// 防止切片越界越界
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// 截取
	data := filteredPolicies[start:end]
	var res []*vo.PolicyVo
	for _, n := range data {
		res = append(res, n)
	}

	var vos []vo.PolicyVo
	for _, n := range res {
		vos = append(vos, *n)
	}

	return &dto.PageResult[vo.PolicyVo]{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Total:    int64(len(allPolicies)),
		List:     vos,
	}, nil
}

func (p policyService) UpdatePolicy(ctx context.Context, policyDto *dto.PeerDto) (*vo.PolicyVo, error) {
	//TODO implement me
	panic("implement me")
}

func (p policyService) CreatePolicy(ctx context.Context, namespace, name, action string, labels map[string]string, ingressRules []v1alpha1.IngressRule, egressRules []v1alpha1.EgressRule) error {
	selector := metav1.LabelSelector{
		MatchLabels: labels,
	}

	policy := buildPolicyFromArgs(namespace, name, selector, ingressRules, egressRules, action)

	return p.client.Create(ctx, &policy)
}

func NewPolicyService(client *resource.Client) PolicyService {
	return &policyService{
		log:    log.GetLogger("policy-service"),
		client: client,
	}
}

func buildPolicyFromArgs(namespace, name string, peerSelector metav1.LabelSelector, IngressRule []v1alpha1.IngressRule, EgressRule []v1alpha1.EgressRule, action string) v1alpha1.WireflowPolicy {
	return v1alpha1.WireflowPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WireflowNetwork",
			APIVersion: "wireflowcontroller.wireflow.run/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.WireflowPolicySpec{
			PeerSelector: peerSelector,
			IngressRule:  IngressRule,
			EgressRule:   EgressRule,
			Action:       action,
			Network:      "wireflow-default-net",
		},
	}
}
