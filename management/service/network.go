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
	"wireflow/management/database"
	"wireflow/management/dto"
	"wireflow/management/repository"
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
	client        *resource.Client
	workspaceRepo *repository.WorkspaceRepository
}

func (s *networkService) ListTokens(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.TokenVo], error) {
	//var tokenList wireflowv1alpha1.WireflowEnrollmentTokenList
	//if err := s.client.List(ctx, &tokenList); err != nil {
	//	return nil, err
	//}
	//
	//var tokens []model.Token
	//for _, token := range tokenList.Items {
	//	tokens = append(tokens, model.Token{
	//		Namespace:  token.Namespace,
	//		Token:      token.Spec.Token,
	//		Expiry:     token.Spec.Expiry,
	//		UsageLimit: token.Spec.UsageLimit,
	//	})
	//}
	//
	//return tokens, nil

	var (
		tokenList wireflowv1alpha1.WireflowEnrollmentTokenList
		err       error
	)

	workspaceV := ctx.Value(infra.WorkspaceKey)
	var workspaceId string
	if workspaceV != nil {
		workspaceId = workspaceV.(string)
	}

	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceId)
	if err != nil {
		return nil, err
	}

	err = s.client.GetAPIReader().List(ctx, &tokenList, client.InNamespace(workspace.Namespace))

	if err != nil {
		return nil, err
	}

	// 2. 获取全量数据（模拟）
	allTokens := []*vo.TokenVo{ /* ... 很多数据 ... */ }

	for _, item := range tokenList.Items {
		allTokens = append(allTokens, &vo.TokenVo{
			Namespace:  item.Namespace,
			Token:      item.Spec.Token,
			Expiry:     item.Spec.Expiry,
			UsageLimit: item.Spec.UsageLimit,
		})
	}

	// 3. 逻辑过滤（搜索）
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

	// 4. 执行内存切片分页
	total := len(filteredTokens)
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
	data := filteredTokens[start:end]
	var res []vo.TokenVo
	for _, n := range data {
		res = append(res, vo.TokenVo{
			Namespace:  n.Namespace,
			Token:      n.Token,
			Expiry:     n.Expiry,
			UsageLimit: n.UsageLimit,
		})
	}

	return &dto.PageResult[vo.TokenVo]{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Total:    int64(len(allTokens)),
		List:     res,
	}, nil
}

func NewNetworkService(client *resource.Client) NetworkService {
	return &networkService{
		client:        client,
		workspaceRepo: repository.NewWorkspaceRepository(database.DB),
	}
}

// TODO implement for wireflow-cli

func (s *networkService) CreateNetwork(ctx context.Context, networkId, cidr string) (*infra.Network, error) {
	network, err := s.client.CreateNetwork(ctx, networkId, cidr)
	if err != nil {
		return nil, err
	}

	return &infra.Network{
		NetworkName: network.Name,
	}, nil

}

// JoinNetwork
func (s *networkService) JoinNetwork(ctx context.Context, appIds []string, networkId string) error {
	//更新
	var err error
	if networkId == "" {
		return nil
	}
	for _, appId := range appIds {
		if err = s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowv1alpha1.WireflowPeer) {
			node.Spec.Network = &networkId
		}); err != nil {
			return err
		}
	}

	return nil
}

// LeaveNetwork
func (s *networkService) LeaveNetwork(ctx context.Context, appIds []string, networkId string) error {
	if networkId == "" {
		return nil
	}
	//更新
	var err error
	for _, appId := range appIds {
		if err = s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowv1alpha1.WireflowPeer) {
			node.Spec.Network = nil
		}); err != nil {
			return err
		}
	}
	return nil

}
