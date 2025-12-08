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

package server

import (
	"context"
	"wireflow/internal"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
)

// TODO implement for wireflow-cli

func (s *Server) CreateNetwork(ctx context.Context, networkId, cidr string) (*internal.Network, error) {
	network, err := s.client.CreateNetwork(ctx, networkId, cidr)
	if err != nil {
		return nil, err
	}

	return &internal.Network{
		NetworkName: network.Name,
	}, nil

}

// JoinNetwork
func (s *Server) JoinNetwork(ctx context.Context, appId, networkId string) error {
	//更新
	if networkId == "" {
		return nil
	}
	return s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowcontrollerv1alpha1.Node) {
		node.Spec.Networks = append(node.Spec.Networks, networkId)
	})
}

// LeaveNetwork
func (s *Server) LeaveNetwork(ctx context.Context, appId, networkId string) error {
	if networkId == "" {
		return nil
	}
	//更新
	return s.client.UpdateNodeSepc(ctx, "default", appId, func(node *wireflowcontrollerv1alpha1.Node) {
		for i, network := range node.Spec.Networks {
			if network == networkId {
				node.Spec.Networks = append(node.Spec.Networks[:i], node.Spec.Networks[i+1:]...)
			}
		}
	})
}
