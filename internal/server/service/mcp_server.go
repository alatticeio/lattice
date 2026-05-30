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

// MCPServerService manages MCPServer CRD CRUD.
type MCPServerService interface {
	List(ctx context.Context, namespace string) ([]v1alpha1.MCPServer, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.MCPServer, error)
	Create(ctx context.Context, namespace string, spec v1alpha1.MCPServerSpec, name string) (*v1alpha1.MCPServer, error)
	Update(ctx context.Context, namespace, name string, spec v1alpha1.MCPServerSpec) (*v1alpha1.MCPServer, error)
	Delete(ctx context.Context, namespace, name string) error
}

type mcpServerService struct {
	k8s k8sclient.Client
}

// NewMCPServerService creates a new MCPServerService.
func NewMCPServerService(k8s k8sclient.Client) MCPServerService {
	return &mcpServerService{k8s: k8s}
}

func (s *mcpServerService) List(ctx context.Context, namespace string) ([]v1alpha1.MCPServer, error) {
	list := &v1alpha1.MCPServerList{}
	if err := s.k8s.List(ctx, list, k8sclient.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list MCPServers: %w", err)
	}
	return list.Items, nil
}

func (s *mcpServerService) Get(ctx context.Context, namespace, name string) (*v1alpha1.MCPServer, error) {
	obj := &v1alpha1.MCPServer{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get MCPServer %s/%s: %w", namespace, name, err)
	}
	return obj, nil
}

func (s *mcpServerService) Create(ctx context.Context, namespace string, spec v1alpha1.MCPServerSpec, name string) (*v1alpha1.MCPServer, error) {
	obj := &v1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       spec,
	}
	if err := s.k8s.Create(ctx, obj); err != nil {
		return nil, fmt.Errorf("create MCPServer: %w", err)
	}
	return obj, nil
}

func (s *mcpServerService) Update(ctx context.Context, namespace, name string, spec v1alpha1.MCPServerSpec) (*v1alpha1.MCPServer, error) {
	obj := &v1alpha1.MCPServer{}
	if err := s.k8s.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get MCPServer for update: %w", err)
	}
	patch := k8sclient.MergeFrom(obj.DeepCopy())
	obj.Spec = spec
	if err := s.k8s.Patch(ctx, obj, patch); err != nil {
		return nil, fmt.Errorf("patch MCPServer: %w", err)
	}
	return obj, nil
}

func (s *mcpServerService) Delete(ctx context.Context, namespace, name string) error {
	obj := &v1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	if err := s.k8s.Delete(ctx, obj); err != nil {
		return fmt.Errorf("delete MCPServer: %w", err)
	}
	return nil
}
