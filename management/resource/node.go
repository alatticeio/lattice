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

// UpdateNodeState
func (n *NodeResource) UpdateNodeState(appId string, status internal.Status) error {
	ctx := context.Background()
	node, err := n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Get(context.Background(), appId, v1.GetOptions{})
	if err != nil {
		klog.Errorf("get node %s failed: %v", appId, err)
		return err
	}

	return n.UpdateCRDWithRetry(ctx, node.Name, func(node *wireflowcontrollerv1alpha1.Node) error {
		updateNode := node.DeepCopy()
		switch status {
		case internal.Active:
			updateNode.Status.Status = "Active"
		case internal.Inactive:
			updateNode.Status.Status = "Inactive"
			break
		}

		_, err = n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Update(ctx, updateNode, v1.UpdateOptions{})
		return err
	})
}

func (n *NodeResource) UpdateCRDWithRetry(ctx context.Context, crdName string, updateFunc func(node *wireflowcontrollerv1alpha1.Node) error) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// 获取最新的 CRD
		crd, err := n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").Get(ctx, crdName, v1.GetOptions{})
		if err != nil {
			return err
		}

		// 应用修改
		return updateFunc(crd)
	})
}

func (n *NodeResource) GetByAppId(ctx context.Context, appId string) (*entity.Node, error) {
	return nil, nil
}
