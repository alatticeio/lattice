package resource

import (
	"context"
	"wireflow/internal"
	"wireflow/management/dto"
	"wireflow/management/entity"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
				AppId:     e.AppID,
				NodeName:  e.Name,
				ClientId:  key.String(),
				PublicKey: key.PublicKey().String(),
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
		PrivateKey: node.Spec.ClientId,
	}, err
}

// UpdateNodeState
func (n *NodeResource) UpdateNodeState(appId string, status internal.Status) error {
	node, err := n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Get(context.Background(), appId, v1.GetOptions{})
	if err != nil {
		klog.Errorf("get node %s failed: %v", appId, err)
		return err
	}

	updateNode := node.DeepCopy()
	switch status {
	case internal.Active:
		updateNode.Status.Status = "Active"
	case internal.Inactive:
		updateNode.Status.Status = "Inactive"
		break
	}

	_, err = n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Update(context.Background(), updateNode, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("update node %s failed: %v", appId, err)
		return err
	}
	klog.Infof("update node %s status to %s successfully", appId, status)

	return nil
}

func (n *NodeResource) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return nil, nil
}
