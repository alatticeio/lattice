package resource

import (
	"context"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkResource struct {
	controller *Controller
}

func NewNetworkResource(controller *Controller) *NetworkResource {
	return &NetworkResource{
		controller: controller,
	}
}

// find all nodes in network
func (n *NetworkResource) FindAll(ctx context.Context) (*wireflowcontrollerv1alpha1.NodeList, error) {
	return n.controller.Clientset.WireflowcontrollerV1alpha1().Nodes("default").List(ctx, v1.ListOptions{})
}
