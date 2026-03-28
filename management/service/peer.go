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
	"fmt"
	"strings"
	"time"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/resource"
	"wireflow/management/vo"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"wireflow/api/v1alpha1"
)

var (
	_ PeerService = (*peerService)(nil)
)

type PeerService interface {
	Register(ctx context.Context, dto *dto.PeerDto) (*infra.Peer, error)
	UpdateStatus(ctx context.Context, status int) error
	GetNetmap(ctx context.Context, namespace string, appId string) (*infra.Message, error)
	CreateToken(ctx context.Context, tokenDto *dto.TokenDto) ([]byte, error)
	bootstrap(ctx context.Context, provideToken string) error

	//Peer tenant
	ListPeers(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PeerVo], error)
	UpdatePeer(ctx context.Context, peerDto *dto.PeerDto) (*vo.PeerVo, error)
}

type peerService struct {
	logger *log.Logger
	client *resource.Client
	store  store.Store
}

func (p *peerService) UpdatePeer(ctx context.Context, peerDto *dto.PeerDto) (*vo.PeerVo, error) {
	var peer v1alpha1.WireflowPeer
	if err := p.client.GetAPIReader().Get(ctx, types.NamespacedName{Namespace: peerDto.Namespace, Name: peerDto.Name}, &peer); err != nil {
		return nil, err
	}

	peerLabels := peer.GetLabels()
	if peerLabels == nil {
		peerLabels = make(map[string]string)
	}

	if peerDto.Labels != nil {
		for k, v := range peerDto.Labels {
			if v == "" {
				delete(peerLabels, k)
			} else {
				peerLabels[k] = v
			}
		}
	}

	peer.SetLabels(peerLabels)

	err := p.client.Update(ctx, &peer)
	if err != nil {
		return nil, err
	}

	return &vo.PeerVo{
		Name:      peer.Name,
		AppID:     peer.Spec.AppId,
		Labels:    peerLabels,
		PublicKey: peer.Spec.PublicKey,
		Platform:  peer.Spec.Platform,
		Address:   peer.Status.AllocatedAddress,
	}, nil
}

func (p *peerService) ListPeers(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.PeerVo], error) {
	var (
		peerList v1alpha1.WireflowPeerList
		err      error
	)

	workspaceV := ctx.Value(infra.WorkspaceKey)
	var workspaceId string
	if workspaceV != nil {
		workspaceId = workspaceV.(string)
	}

	workspace, err := p.store.Workspaces().GetByID(ctx, workspaceId)
	if err != nil {
		return nil, err
	}

	err = p.client.GetAPIReader().List(ctx, &peerList, client.InNamespace(workspace.Namespace))
	if err != nil {
		return nil, err
	}

	allPeers := []*models.Peer{}

	for _, n := range peerList.Items {
		allPeers = append(allPeers, &models.Peer{
			Name:       n.Name,
			PublicKey:  n.Spec.PublicKey,
			AppID:      n.Spec.AppId,
			PrivateKey: n.Spec.PrivateKey,
			Labels:     n.GetLabels(),
		})
	}

	var filteredNodes []*models.Peer
	if pageParam.Keyword != "" {
		for _, n := range allPeers {
			if strings.Contains(n.Name, pageParam.Keyword) || (n.Address != nil && strings.Contains(*n.Address, pageParam.Keyword)) {
				filteredNodes = append(filteredNodes, n)
			}
		}
	} else {
		filteredNodes = allPeers
	}

	total := len(filteredNodes)
	start := (pageParam.Page - 1) * pageParam.PageSize
	end := start + pageParam.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var vos []vo.PeerVo
	for _, n := range filteredNodes[start:end] {
		vos = append(vos, vo.PeerVo{
			Name:      n.Name,
			PublicKey: n.PublicKey,
			AppID:     n.AppID,
			Labels:    n.Labels,
		})
	}

	return &dto.PageResult[vo.PeerVo]{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Total:    int64(len(allPeers)),
		List:     vos,
	}, nil
}

func (p *peerService) CreateToken(ctx context.Context, tokenDto *dto.TokenDto) ([]byte, error) {
	var token v1alpha1.WireflowEnrollmentToken
	if err := p.client.Get(ctx, client.ObjectKey{Namespace: tokenDto.Namespace, Name: tokenDto.Name}, &token); err != nil {
		if errors.IsNotFound(err) {
			duration, err := time.ParseDuration(tokenDto.Expiry)
			if err != nil {
				return nil, err
			}

			if err = p.bootstrap(ctx, tokenDto.Namespace); err != nil {
				return nil, err
			}

			expiryTimestamp := time.Now().Add(duration).Unix()

			token = v1alpha1.WireflowEnrollmentToken{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.ToLower(tokenDto.Name),
					Namespace: tokenDto.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "wireflow-controller",
					},
				},
				Spec: v1alpha1.WireflowEnrollmentTokenSpec{
					Token:      tokenDto.Name,
					Namespace:  tokenDto.Namespace,
					Expiry:     metav1.NewTime(time.Unix(expiryTimestamp, 0)),
					UsageLimit: tokenDto.Limit,
				},
			}

			if err = p.client.Create(ctx, &token); err != nil {
				return nil, err
			}
		}
	}

	return []byte(token.Spec.Token), nil
}

func NewPeerService(client *resource.Client, st store.Store) PeerService {
	return &peerService{
		client: client,
		logger: log.GetLogger("peer-service"),
		store:  st,
	}
}

func (p *peerService) GetNetmap(ctx context.Context, token string, appId string) (*infra.Message, error) {
	return p.client.GetNetworkMap(ctx, token, appId)
}

func (p *peerService) UpdateStatus(ctx context.Context, status int) error {
	//TODO implement me
	panic("implement me")
}

func (p *peerService) Register(ctx context.Context, dto *dto.PeerDto) (*infra.Peer, error) {
	p.logger.Info("Received peer", "info", dto)

	tokenValid, token, err := p.checkToken(ctx, dto.Token)
	if err != nil {
		return nil, err
	}

	if !tokenValid {
		return nil, fmt.Errorf("token is invalid")
	}

	node, err := p.client.Register(ctx, token.Namespace, dto)
	if err != nil {
		return nil, err
	}

	node.Token = token.Spec.Token
	return node, nil
}

func (p *peerService) checkToken(ctx context.Context, tokenStr string) (bool, *v1alpha1.WireflowEnrollmentToken, error) {
	if tokenStr == "" {
		return false, nil, fmt.Errorf("token is empty")
	}

	var list v1alpha1.WireflowEnrollmentTokenList
	err := p.client.List(ctx, &list, client.MatchingFields{"spec.token": tokenStr})
	if err != nil {
		return false, nil, fmt.Errorf("get token failed: %v", err)
	}

	if len(list.Items) == 0 {
		return false, nil, fmt.Errorf("Token not exists")
	}

	var token *v1alpha1.WireflowEnrollmentToken
	for _, t := range list.Items {
		if t.Spec.Token == tokenStr {
			token = &t
		}
	}

	if token == nil {
		return false, nil, fmt.Errorf("Token not exists")
	}

	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestToken := &v1alpha1.WireflowEnrollmentToken{}
		if err = p.client.GetCache().Get(ctx, client.ObjectKeyFromObject(token), latestToken); err != nil {
			return err
		}
		latestToken.Status.UsedCount++
		return p.client.Status().Update(ctx, latestToken)
	}); err != nil {
		return false, nil, err
	}

	return true, token, nil
}

func (p *peerService) bootstrap(ctx context.Context, nsName string) error {
	if err := p.ensureNamespace(ctx, nsName); err != nil {
		return err
	}
	return p.ensureDefaultNetwork(ctx, nsName)
}

func (p *peerService) ensureNamespace(ctx context.Context, nsName string) error {
	var ns corev1.Namespace
	if err := p.client.Get(ctx, client.ObjectKey{Name: nsName}, &ns); err != nil {
		if errors.IsNotFound(err) {
			p.logger.Info("Creating namespace", "name", nsName)
			if err = p.client.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nsName,
					Labels: map[string]string{"app.kubernetes.io/managed-by": "wireflow-controller"},
				},
			}); err != nil {
				p.logger.Error("create namespace failed", err)
			}
		}
	}
	return nil
}

func (p *peerService) ensureDefaultNetwork(ctx context.Context, nsName string) error {
	var defaultNet v1alpha1.WireflowNetwork
	if err := p.client.Get(ctx, client.ObjectKey{Namespace: nsName, Name: "wireflow-default-net"}, &defaultNet); err != nil {
		if errors.IsNotFound(err) {
			defaultNet = v1alpha1.WireflowNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireflow-default-net",
					Namespace: nsName,
					Labels:    map[string]string{"app.kubernetes.io/managed-by": "wireflow-controller"},
				},
				Spec: v1alpha1.WireflowNetworkSpec{
					Name: fmt.Sprintf("%s-net", nsName),
				},
			}

			if err := p.client.Create(ctx, &defaultNet); err != nil {
				return fmt.Errorf("failed to create default network: %v", err)
			}
		}
	}
	return nil
}
