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

package controller

import (
	"context"
	"encoding/json"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/management/service"
)

type NetworkController interface {
	CreateNetwork(ctx context.Context, request []byte) ([]byte, error)
	JoinNetwork(ctx context.Context, request []byte) error
	LeaveNetwork(ctx context.Context, request []byte) error
}

type networkController struct {
	networkService service.NetworkService
	policyService  service.PolicyService
}

func (n *networkController) CreateNetwork(ctx context.Context, request []byte) ([]byte, error) {
	var (
		err        error
		networkDto dto.NetworkDto
	)
	if err = json.Unmarshal(request, &networkDto); err != nil {
		return nil, err
	}

	network, err := n.networkService.CreateNetwork(ctx, networkDto.Name, networkDto.CIDR)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(network)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (n networkController) JoinNetwork(ctx context.Context, request []byte) error {
	var (
		err        error
		networkDto dto.NetworkDto
	)
	if err = json.Unmarshal(request, &networkDto); err != nil {
		return err
	}

	err = n.networkService.JoinNetwork(ctx, networkDto.AppIds, networkDto.Name)
	if err != nil {
		return err
	}

	return nil
}

func (n networkController) LeaveNetwork(ctx context.Context, req []byte) error {
	var (
		err        error
		networkDto dto.NetworkDto
	)
	if err = json.Unmarshal(req, &networkDto); err != nil {
		return err
	}

	err = n.networkService.LeaveNetwork(ctx, networkDto.AppIds, networkDto.Name)
	if err != nil {
		return err
	}

	return nil
}

func NewNetworkController(client *resource.Client) NetworkController {
	return &networkController{
		networkService: service.NewNetworkService(client),
		policyService:  service.NewPolicyService(client),
	}
}
