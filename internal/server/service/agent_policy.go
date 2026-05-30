// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"fmt"

	"github.com/alatticeio/lattice/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// AgentPolicyService manages AgentPolicy CRD CRUD.
type AgentPolicyService interface {
	List(ctx context.Context, namespace string) ([]v1alpha1.AgentPolicy, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentPolicy, error)
	Create(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error)
	Update(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error)
	Delete(ctx context.Context, namespace, name string) error
}

type agentPolicyService struct {
	k8s k8sclient.Client
}

// NewAgentPolicyService creates a new AgentPolicyService.
func NewAgentPolicyService(k8s k8sclient.Client) AgentPolicyService {
	return &agentPolicyService{k8s: k8s}
}

func (s *agentPolicyService) List(ctx context.Context, namespace string) ([]v1alpha1.AgentPolicy, error) {
	list := &v1alpha1.AgentPolicyList{}
	if err := s.k8s.List(ctx, list, k8sclient.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list AgentPolicies: %w", err)
	}
	return list.Items, nil
}

func (s *agentPolicyService) Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentPolicy, error) {
	obj := &v1alpha1.AgentPolicy{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get AgentPolicy %s/%s: %w", namespace, name, err)
	}
	return obj, nil
}

func (s *agentPolicyService) Create(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error) {
	obj := &v1alpha1.AgentPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       spec,
	}
	if err := s.k8s.Create(ctx, obj); err != nil {
		return nil, fmt.Errorf("create AgentPolicy: %w", err)
	}
	return obj, nil
}

func (s *agentPolicyService) Update(ctx context.Context, namespace, name string, spec v1alpha1.AgentPolicySpec) (*v1alpha1.AgentPolicy, error) {
	obj := &v1alpha1.AgentPolicy{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get AgentPolicy for update: %w", err)
	}
	patch := k8sclient.MergeFrom(obj.DeepCopy())
	obj.Spec = spec
	if err := s.k8s.Patch(ctx, obj, patch); err != nil {
		return nil, fmt.Errorf("patch AgentPolicy: %w", err)
	}
	return obj, nil
}

func (s *agentPolicyService) Delete(ctx context.Context, namespace, name string) error {
	obj := &v1alpha1.AgentPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	if err := s.k8s.Delete(ctx, obj); err != nil {
		return fmt.Errorf("delete AgentPolicy: %w", err)
	}
	return nil
}
