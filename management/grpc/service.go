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

package grpc

//
//import (
//	"context"
//	"wireflow/internal/core/domain"
//
//	wireflowv1alpha1 "wireflow/api/v1alpha1"
//)
//
//// TODO implement for wireflow-cli
//
//func (s *Server) CreateNetwork(ctx context.Context, networkId, cidr string) (*domain.Network, error) {
//	network, err := s.client.CreateNetwork(ctx, networkId, cidr)
//	if err != nil {
//		return nil, err
//	}
//
//	return &domain.Network{
//		NetworkName: network.Name,
//	}, nil
//
//}
//
//// JoinNetwork
//func (s *Server) JoinNetwork(ctx context.Context, appIds []string, networkId string) error {
//	//更新
//	var err error
//	if networkId == "" {
//		return nil
//	}
//	for _, appId := range appIds {
//		if err = s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowv1alpha1.Node) {
//			node.Spec.Network = &networkId
//		}); err != nil {
//			return err
//		}
//	}
//
//	return nil
//}
//
//// LeaveNetwork
//func (s *Server) LeaveNetwork(ctx context.Context, appIds []string, networkId string) error {
//	if networkId == "" {
//		return nil
//	}
//	//更新
//	var err error
//	for _, appId := range appIds {
//		if err = s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowv1alpha1.Node) {
//			node.Spec.Network = nil
//		}); err != nil {
//			return err
//		}
//	}
//	return nil
//
//}
