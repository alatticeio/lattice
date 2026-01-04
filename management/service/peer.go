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
	"wireflow/internal/core/domain"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/resource"
)

var (
	_ PeerService = (*peerService)(nil)
)

type PeerService interface {
	Register(ctx context.Context, dto *dto.PeerDto) (*domain.Peer, error)
	UpdateStatus(ctx context.Context, status int) error
	GetNetmap(ctx context.Context, namespace string, appId string) (*domain.Message, error)
}

type peerService struct {
	logger *log.Logger
	client *resource.Client
}

func NewPeerService(client *resource.Client) PeerService {
	return &peerService{
		client: client,
		logger: log.NewLogger(log.Loglevel, "peer-service"),
	}
}

func (p *peerService) GetNetmap(ctx context.Context, namespace string, appId string) (*domain.Message, error) {
	return p.client.GetNetworkMap(ctx, namespace, appId)
}

func (p *peerService) UpdateStatus(ctx context.Context, status int) error {
	//TODO implement me
	panic("implement me")
}

func (p *peerService) Register(ctx context.Context, dto *dto.PeerDto) (*domain.Peer, error) {
	p.logger.Infof("Received peer info: %+v", dto)
	node, err := p.client.Register(ctx, dto)

	if err != nil {
		return nil, err
	}
	return node, nil
}
