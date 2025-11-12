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
