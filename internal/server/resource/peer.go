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

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *Client) Register(ctx context.Context, namespace string, e *dto.PeerDto) (*infra.Peer, error) {
	log := logf.FromContext(ctx)
	log.Info("Register node", "node", e)
	var (
		node v1alpha1.LatticePeer
		err  error
		key  wgtypes.Key
	)

	log.Info("Get node", "node", e)
	err = c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      e.AppID,
	}, &node)

	var peerId infra.PeerID
	if err != nil && errors.IsNotFound(err) {
		key, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
	} else {
		key, err = wgtypes.ParseKey(node.Spec.PrivateKey)
		if err != nil {
			return nil, err
		}
	}

	peerId = infra.FromKey(key.PublicKey())

	log.Info("Updating default net...")
	// 使用SSA模式
	manager := client.FieldOwner("lattice-controller-manager")

	defaultNet := "lattice-default-net"
	node = v1alpha1.LatticePeer{
		TypeMeta: v1.TypeMeta{
			Kind:       "LatticePeer",
			APIVersion: "alattice.io/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: namespace,
			Name:      e.AppID,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "lattice-controller",
			},
		},
		Spec: v1alpha1.LatticePeerSpec{
			Network:       &defaultNet,
			AppId:         e.AppID,
			Platform:      e.Platform,
			InterfaceName: e.InterfaceName,
			PrivateKey:    key.String(),
			PublicKey:     key.PublicKey().String(),
			PeerId:        fmt.Sprintf("%d", peerId.ToUint64()),
		},

		Status: v1alpha1.LatticePeerStatus{
			Status: "Inactive",
		},
	}

	if err = c.Patch(ctx, &node, client.Apply, manager); err != nil {
		return nil, err
	}

	log.Info("Register node success", "node", node)
	return &infra.Peer{
		AppID:      node.Spec.AppId,
		Address:    node.Status.AllocatedAddress,
		PrivateKey: node.Spec.PrivateKey,
		PublicKey:  node.Spec.PublicKey,
		PeerID:     peerId.ToUint64(),
		NetworkId:  namespace,
	}, err
}

// UpdateNodeStatus used to update node status
func (c *Client) UpdateNodeStatus(ctx context.Context, namespace, name string, updateFunc func(status *v1alpha1.LatticePeerStatus)) error {
	logger := logf.FromContext(ctx)
	logger.Info("Update node status", "namespace", namespace, "name", name)

	var node v1alpha1.LatticePeer
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &node); err != nil {
		return err
	}

	updateFunc(&node.Status)

	return c.Status().Update(ctx, &node)
}

func (c *Client) UpdateNodeSepc(ctx context.Context, namespace, name string, updateFunc func(node *v1alpha1.LatticePeer)) error {
	logger := logf.FromContext(ctx)
	logger.Info("Update node spec", "namespace", namespace, "name", name)
	var node v1alpha1.LatticePeer
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &node); err != nil {
		return err
	}
	updateFunc(&node)
	return c.Update(ctx, &node)
}

// GetNetworkMap get network map when node init
func (c *Client) GetNetworkMap(ctx context.Context, tokenStr, name string) (*infra.Message, error) {
	logger := c.log
	logger.Info("Get node", "tokenStr", tokenStr, "name", name)

	if tokenStr == "" {
		return nil, fmt.Errorf("token is empty")
	}
	var list v1alpha1.LatticeEnrollmentTokenList
	err := c.List(ctx, &list, client.MatchingFields{"spec.token": tokenStr})
	if err != nil {
		return nil, fmt.Errorf("get token failed: %v", err)
	}

	if len(list.Items) == 0 {
		return nil, fmt.Errorf("token not exists")
	}

	var token *v1alpha1.LatticeEnrollmentToken
	for _, t := range list.Items {
		if t.Status.Token == tokenStr {
			token = &t
		}
	}

	if token == nil {
		return nil, fmt.Errorf("token not exists")
	}

	var node v1alpha1.LatticePeer
	if err = c.Get(ctx, types.NamespacedName{Namespace: token.Namespace, Name: name}, &node); err != nil {
		return nil, err
	}

	//从network获取
	var nodeConfig corev1.ConfigMap
	if err = c.Get(ctx, types.NamespacedName{
		Namespace: node.Namespace,
		Name:      fmt.Sprintf("%s-config", node.Name),
	}, &nodeConfig); err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap 尚未被 controller 创建（节点首次启动时的正常情况）。
			// 返回空 Message，agent 以空配置启动，后续通过 NATS 推送接收完整配置。
			logger.Info("ConfigMap not found yet, returning empty network map", "namespace", node.Namespace, "name", node.Name)
			return &infra.Message{}, nil
		}
		return nil, err
	}

	data := nodeConfig.Data["config.json"]
	var message *infra.Message
	err = json.Unmarshal([]byte(data), &message)
	if err != nil {
		return nil, err
	}

	logger.Info("Get network map success", "namespace", token.Namespace, "name", name, "message", message)
	return message, nil
}

// CreateNetwork create a network
func (c *Client) CreateNetwork(ctx context.Context, networkId, cidr string) (*v1alpha1.LatticeNetwork, error) {
	var (
		err     error
		network v1alpha1.LatticeNetwork
	)
	err = c.Get(ctx, types.NamespacedName{
		Namespace: "default",
		Name:      networkId,
	}, &network)

	if err != nil && errors.IsNotFound(err) {
		// 使用SSA模式
		manager := client.FieldOwner("lattice-controller-manager")

		if err = c.Patch(ctx, &v1alpha1.LatticeNetwork{
			TypeMeta: v1.TypeMeta{
				Kind:       "LatticeNetwork",
				APIVersion: "alattice.io/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Name:      networkId,
			},
			Spec: v1alpha1.LatticeNetworkSpec{
				Name: networkId,
				CIDR: cidr,
			},
		}, client.Apply, manager); err != nil {
			return nil, err
		}
	}

	return &network, nil
}
