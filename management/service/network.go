// Copyright 2025 The Wireflow Authors, Inc.
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
	"strings"
	wireflowv1alpha1 "wireflow/api/v1alpha1"
	"wireflow/internal/infra"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/management/vo"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NetworkService interface {
	CreateNetwork(ctx context.Context, networkId, cidr string) (*infra.Network, error)
	JoinNetwork(ctx context.Context, appIds []string, networkId string) error
	LeaveNetwork(ctx context.Context, appIds []string, networkId string) error
	ListTokens(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.TokenVo], error)
}

type networkService struct {
	client *resource.Client
	store  store.Store
}

func (s *networkService) ListTokens(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.TokenVo], error) {
	var (
		tokenList wireflowv1alpha1.WireflowEnrollmentTokenList
		err       error
	)

	workspaceV := ctx.Value(infra.WorkspaceKey)
	var workspaceId string
	if workspaceV != nil {
		workspaceId = workspaceV.(string)
	}

	workspace, err := s.store.Workspaces().GetByID(ctx, workspaceId)
	if err != nil {
		return nil, err
	}

	err = s.client.GetAPIReader().List(ctx, &tokenList, client.InNamespace(workspace.Namespace))
	if err != nil {
		return nil, err
	}

	allTokens := []*vo.TokenVo{}

	for _, item := range tokenList.Items {
		workspaceDisplayName := ""
		if ws, err := s.store.Workspaces().GetByNamespace(ctx, item.Namespace); err == nil && ws != nil {
			workspaceDisplayName = ws.DisplayName
		}
		allTokens = append(allTokens, &vo.TokenVo{
			Namespace:            item.Namespace,
			WorkspaceDisplayName: workspaceDisplayName,
			Token:                item.Status.Token,
			Expiry:               item.Spec.Expiry,
			UsageLimit:           item.Spec.UsageLimit,
			BoundPeers:           item.Status.BoundPeers,
			UsedCount:            item.Status.UsedCount,
			IsExpired:            item.Status.IsExpired,
			Phase:                item.Status.Phase,
		})
	}

	var filteredTokens []*vo.TokenVo
	if pageParam.Keyword != "" {
		for _, n := range allTokens {
			if strings.Contains(n.Token, pageParam.Keyword) {
				filteredTokens = append(filteredTokens, n)
			}
		}
	} else {
		filteredTokens = allTokens
	}

	total := len(filteredTokens)
	start := (pageParam.Page - 1) * pageParam.PageSize
	end := start + pageParam.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var res []vo.TokenVo
	for _, n := range filteredTokens[start:end] {
		res = append(res, *n)
	}

	return &dto.PageResult[vo.TokenVo]{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Total:    int64(len(allTokens)),
		List:     res,
	}, nil
}

func NewNetworkService(client *resource.Client, st store.Store) NetworkService {
	return &networkService{
		client: client,
		store:  st,
	}
}

func (s *networkService) CreateNetwork(ctx context.Context, networkId, cidr string) (*infra.Network, error) {
	network, err := s.client.CreateNetwork(ctx, networkId, cidr)
	if err != nil {
		return nil, err
	}
	return &infra.Network{NetworkName: network.Name}, nil
}

func (s *networkService) JoinNetwork(ctx context.Context, appIds []string, networkId string) error {
	if networkId == "" {
		return nil
	}
	for _, appId := range appIds {
		if err := s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowv1alpha1.WireflowPeer) {
			node.Spec.Network = &networkId
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *networkService) LeaveNetwork(ctx context.Context, appIds []string, networkId string) error {
	if networkId == "" {
		return nil
	}
	for _, appId := range appIds {
		if err := s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowv1alpha1.WireflowPeer) {
			node.Spec.Network = nil
		}); err != nil {
			return err
		}
	}
	return nil
}
