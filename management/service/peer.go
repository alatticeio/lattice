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
	"crypto/rand"
	"fmt"
	"math/big"
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	bootstrap(ctx context.Context, provideToken string) (string, string, error)
}

type peerService struct {
	logger *log.Logger
	client *resource.Client
}

func (p *peerService) Join(ctx context.Context, dto *dto.PeerDto) (*infra.Peer, error) {
	return nil, nil
}

func NewPeerService(client *resource.Client) PeerService {
	return &peerService{
		client: client,
		logger: log.GetLogger("peer-service"),
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

	//handle bootstrap
	ns, token, err := p.bootstrap(ctx, dto.Token)
	if err != nil {
		return nil, err
	}

	node, err := p.client.Register(ctx, ns, dto)

	if err != nil {
		return nil, err
	}

	// setToken if bootstrap success
	node.Token = token
	return node, nil
}

func (p *peerService) bootstrap(ctx context.Context, providedToken string) (string, string, error) {
	var err error
	if providedToken == "" {
		providedToken, err = GenerateSecureToken()
		if err != nil {
			return "", "", err
		}
	}

	nsName := utils.DeriveNamespace(providedToken)
	secretName := "wireflow-auth"

	// 1. 获取或创建 Namespace
	var ns corev1.Namespace
	err = p.client.GetAPIReader().Get(ctx, client.ObjectKey{Name: nsName}, &ns)

	if errors.IsNotFound(err) {
		p.logger.Info("Creating namespace", "name", nsName, "token", providedToken)
		// 创建 Namespace
		if err = p.client.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "wireflow-controller",
				},
			},
		}); err != nil {
			return "", "", err
		}

		// 创建 Secret 存储 Token
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: nsName},
			Data:       map[string][]byte{"token": []byte(providedToken)},
		}
		if err = p.client.Create(ctx, secret); err != nil {
			return "", "", err
		}

		p.logger.Info("Namespace created", "name", nsName, "secret", secretName, "token", providedToken, "separator", "")

		// 初始化网络资源 (WireflowNetwork)
		var defaultNet string
		if defaultNet, err = p.ensureDefaultNetwork(ctx, nsName); err != nil {
			p.logger.Error("ensure default network failed", err)
			return "", "", err
		} else {
			p.logger.Info("default network created", "defaultNetwork", defaultNet, "separator", "")
		}

		p.logger.Info("Bootstrap success", "name", nsName, "secret", secretName, "token", providedToken, "defaultNet", defaultNet, "separator", "")
		// 返回给 Agent：你是创建者，这是你的新 Token
		return nsName, providedToken, nil
	}

	// --- 房客模式：验证已有空间 ---
	var authSecret corev1.Secret
	if err = p.client.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: nsName, Name: secretName}, &authSecret); err != nil {
		return "", "", err
	}

	storedToken := string(authSecret.Data["token"])
	if providedToken != storedToken {
		return "", "", fmt.Errorf("invalid token")
	}

	return nsName, storedToken, nil
}

// EnsureNamespaceForPeer 为新接入的节点确保环境就绪
// token: 节点生成的唯一标识（可以是 hash 后的公钥）
func (p *peerService) ensureDefaultNetwork(ctx context.Context, nsName string) (string, error) {

	defaultNet := &v1alpha1.WireflowNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wireflow-default-net",
			Namespace: nsName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wireflow-controller", // 必须对应
			},
		},
		Spec: v1alpha1.WireflowNetworkSpec{
			Name: fmt.Sprintf("%s-net", nsName),
		},
	}

	if err := p.client.Create(ctx, defaultNet); err != nil {
		return "", fmt.Errorf("failed to create default network: %v", err)
	}

	return defaultNet.Name, nil
}

func GenerateSecureToken() (string, error) {
	// 定义 Token 可能包含的字符（去掉了容易混淆的字符如 0, O, I, l）
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	length := 16
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		// 生成一个随机索引
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}
