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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_ PeerController = (*peerController)(nil)
)

type PeerController interface {
	Register(ctx context.Context, request []byte) ([]byte, error)
	GetNetmap(ctx context.Context, request []byte) ([]byte, error)
	UpdateStatus(ctx context.Context, status int) error
}

func NewPeerController(client *resource.Client) PeerController {
	return &peerController{
		peerService: service.NewPeerService(client),
	}
}

type peerController struct {
	peerService service.PeerService
}

func (p *peerController) UpdateStatus(ctx context.Context, status int) error {
	//TODO implement me
	panic("implement me")
}

func (p *peerController) Register(ctx context.Context, request []byte) ([]byte, error) {
	var req dto.PeerDto
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, err
	}
	peer, err := p.peerService.Register(context.Background(), &req)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(peer)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p *peerController) GetNetmap(ctx context.Context, request []byte) ([]byte, error) {
	var (
		peer dto.PeerDto
		err  error
	)
	if err = json.Unmarshal(request, &peer); err != nil {
		return nil, err
	}
	networkMap, err := p.peerService.GetNetmap(ctx, "default", peer.AppID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(networkMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
	}
	return data, nil
}
