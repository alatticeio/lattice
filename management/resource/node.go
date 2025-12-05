// Copyright 2025 wireflowio.com, Inc.
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
	"wireflow/internal"
	"wireflow/management/dto"
	"wireflow/management/entity"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *Client) Register(ctx context.Context, e *dto.NodeDto) (*internal.Peer, error) {
	log := logf.FromContext(ctx)
	log.Info("Register node", "node", e)
	var (
		node wireflowv1alpha1.Node
		err  error
		key  wgtypes.Key
	)

	err = c.client.Get(ctx, types.NamespacedName{
		Namespace: "default",
		Name:      e.AppID,
	}, &node)

	if err != nil && errors.IsNotFound(err) {
		key, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}

		// 使用SSA模式
		manager := client.FieldOwner("wireflow-controller-manager")

		if err = c.client.Patch(ctx, &wireflowv1alpha1.Node{
			TypeMeta: v1.TypeMeta{
				Kind:       "Node",
				APIVersion: "wireflowcontroller.wireflow.io/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Name:      e.AppID,
			},
			Spec: wireflowv1alpha1.NodeSpec{
				AppId:      e.AppID,
				PrivateKey: key.String(),
				PublicKey:  key.PublicKey().String(),
			},

			Status: wireflowv1alpha1.NodeStatus{
				Status: "Active",
			},
		}, client.Apply, manager); err != nil {
			return nil, err
		}
	}

	if err = c.client.Get(ctx, types.NamespacedName{
		Namespace: "default",
		Name:      e.AppID,
	}, &node); err != nil {
		return nil, err
	}

	return &internal.Peer{
		AppID:      node.Spec.AppId,
		Address:    node.Status.AllocatedAddress,
		PrivateKey: node.Spec.PrivateKey,
		PublicKey:  node.Spec.PublicKey,
	}, err
}

// UpdateNodeStatus used to update node status
func (c *Client) UpdateNodeStatus(ctx context.Context, namespace, name string, updateFunc func(status *wireflowv1alpha1.NodeStatus)) error {
	logger := logf.FromContext(ctx)
	logger.Info("Update node status", "namespace", namespace, "name", name)

	var node wireflowv1alpha1.Node
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &node); err != nil {
		return err
	}

	updateFunc(&node.Status)

	return c.client.Status().Update(ctx, &node)
}

func (c *Client) UpdateNodeSepc(ctx context.Context, namespace, name string, updateFunc func(node *wireflowv1alpha1.Node)) error {
	logger := logf.FromContext(ctx)
	logger.Info("Update node sepc", "namespace", namespace, "name", name)
	var node wireflowv1alpha1.Node
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &node); err != nil {
		return err
	}
	updateFunc(&node)
	return c.client.Update(ctx, &node)
}

// GetNetworkMap get network map when node init
func (c *Client) GetNetworkMap(ctx context.Context, namespace, name string) (*internal.Message, error) {
	logger := c.log
	logger.Infof("Get node, namespace: %s, name: %s", namespace, name)

	var node wireflowv1alpha1.Node
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &node); err != nil {
		return nil, err
	}

	//从network获取
	var nodeConfig corev1.ConfigMap
	if err := c.client.Get(ctx, types.NamespacedName{
		Namespace: node.Namespace,
		Name:      fmt.Sprintf("%s-config", node.Name),
	}, &nodeConfig); err != nil {
		return nil, err
	}

	data := nodeConfig.Data["config.json"]
	var message *internal.Message
	err := json.Unmarshal([]byte(data), &message)
	if err != nil {
		return nil, err
	}

	logger.Infof("Get network map success, namespace: %s, name: %s, messsage: %v", namespace, name, message)
	return message, nil
}

func (c *Client) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return nil, nil
}

// CreateNetwork create a network
func (c *Client) CreateNetwork(ctx context.Context, networkId, cidr string) (*wireflowv1alpha1.Network, error) {
	var (
		err     error
		network wireflowv1alpha1.Network
	)
	err = c.client.Get(ctx, types.NamespacedName{
		Namespace: "default",
		Name:      networkId,
	}, &network)

	if err != nil && errors.IsNotFound(err) {
		// 使用SSA模式
		manager := client.FieldOwner("wireflow-controller-manager")

		if err = c.client.Patch(ctx, &wireflowv1alpha1.Network{
			TypeMeta: v1.TypeMeta{
				Kind:       "Network",
				APIVersion: "wireflowcontroller.wireflow.io/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Name:      networkId,
			},
			Spec: wireflowv1alpha1.NetworkSpec{
				Name: networkId,
				CIDR: cidr,
			},
		}, client.Apply, manager); err != nil {
			return nil, err
		}
	}

	return &network, nil
}
