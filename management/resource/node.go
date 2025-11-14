// Copyright 2025 Wireflow.io, Inc.
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
	"fmt"
	"wireflow/internal"
	"wireflow/management/dto"
	"wireflow/management/entity"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"k8s.io/client-go/util/retry"
)

type NodeResource struct {
	controller *Controller
}

func NewNodeResource(controller *Controller) *NodeResource {
	return &NodeResource{
		controller: controller,
	}
}

func (n *NodeResource) Register(ctx context.Context, e *dto.NodeDto) (*internal.Node, error) {

	var (
		node *wireflowcontrollerv1alpha1.Node
		err  error
		key  wgtypes.Key
	)

	node, err = n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Get(ctx, e.AppID, v1.GetOptions{})

	if node == nil || err != nil {
		key, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}

		if node, err = n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Create(ctx, &wireflowcontrollerv1alpha1.Node{
			TypeMeta: v1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: e.AppID,
			},
			Spec: wireflowcontrollerv1alpha1.NodeSpec{
				Address:    e.Address,
				AppId:      e.AppID,
				NodeName:   e.Name,
				PrivateKey: key.String(),
				PublicKey:  key.PublicKey().String(),
			},

			Status: wireflowcontrollerv1alpha1.NodeStatus{
				Status: "Active",
			},
		}, v1.CreateOptions{}); err != nil {
			return nil, err
		}
	} else {
		klog.Infof("node %s already exists", node.Name)
	}

	return &internal.Node{
		AppID:      node.Spec.AppId,
		Address:    node.Spec.Address,
		PrivateKey: node.Spec.PrivateKey,
		PublicKey:  node.Spec.PublicKey,
	}, err
}

// UpdateNodeStatus used to update node status
func (n *NodeResource) UpdateNodeStatus(ctx context.Context, namespace, name string, updateFunc func(status *wireflowcontrollerv1alpha1.NodeStatus)) error {
	logger := klog.FromContext(ctx)
	logger.Info("Update node status", "namespace", namespace, "name", name)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		node, err := n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes(namespace).Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get node %s failed: %v", name, err)
		}

		nodeCopy := node.DeepCopy()
		updateFunc(&nodeCopy.Status)

		_, err = n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes(namespace).UpdateStatus(ctx, nodeCopy, v1.UpdateOptions{})

		if err != nil {
			if errors.IsConflict(err) {
				return nil
			}
			return fmt.Errorf("update node %s status failed: %v", name, err)
		}
		return err

	})
}

// UpdateNetworkSpec union update network spec
func (n *NodeResource) UpdateNetworkSpec(ctx context.Context, namespace, name string, updateFunc func(spec *wireflowcontrollerv1alpha1.Network)) error {
	logger := klog.FromContext(ctx)
	logger.Info("Update node status", "namespace", namespace, "name", name)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		network, err := n.controller.networkLister.Networks(namespace).Get(name)
		if err != nil {
			return fmt.Errorf("get node %s failed: %v", name, err)
		}

		networkCopy := network.DeepCopy()
		updateFunc(networkCopy)

		_, err = n.controller.Clientset.WireflowcontrollerV1alpha1().Networks(namespace).Update(ctx, networkCopy, v1.UpdateOptions{})

		if err != nil {
			if errors.IsConflict(err) {
				return nil
			}
			return fmt.Errorf("update network %s failed: %v", name, err)
		}
		return err

	})
}

// GetNetworkMap get network map when node init
func (n *NodeResource) GetNetworkMap(ctx context.Context, namespace, name string) (*internal.Message, error) {
	logger := klog.FromContext(ctx)
	logger.Info("Get node", "namespace", namespace, "name", name)
	node, err := n.controller.nodeLister.Nodes(namespace).Get(name)
	if err != nil {
		return nil, err
	}

	context := nodeContext(node, n.controller.nodeLister, n.controller.networkLister, n.controller.policyLister)

	return buildFullConfig(node, context, nil, "init")
}

func (n *NodeResource) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return nil, nil
}
