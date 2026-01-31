package service

import (
	"context"
	"wireflow/api/v1alpha1"
	"wireflow/internal/log"
	"wireflow/management/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicyService interface {
	CreatePolicy(ctx context.Context, namespace, name, action string, labels map[string]string, ingressRules []v1alpha1.IngressRule, egressRules []v1alpha1.EgressRule) error
}

type policyService struct {
	log    *log.Logger
	client *resource.Client
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
