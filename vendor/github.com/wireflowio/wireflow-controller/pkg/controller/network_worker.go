package controller

import (
	"context"
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// nodeQueue.
func (c *Controller) runNetworkWorker(ctx context.Context) {
	for c.processNextWorkNetwork(ctx) {
	}
}

// processNextWorkItem will read a single work item off the nodeQueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkNetwork(ctx context.Context) bool {
	item, shutdown := c.networkQueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We call Done at the end of this func so the nodeQueue knows we have
	// finished processing this item. We also must remember to call Forget
	// if we do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the nodeQueue and attempted again after a back-off
	// period.
	defer c.networkQueue.Done(item)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	err := c.syncNetworkHandler(ctx, item)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		c.networkQueue.Forget(item)
		logger.Info("Successfully synced", "objectName", item)
		return true
	}
	// there was a failure so be sure to report it.  This method allows for
	// pluggable error handling which can be used for things like
	// cluster-monitoring.
	utilruntime.HandleErrorWithContext(ctx, err, "Error syncing; requeuing for later retry", "objectReference", item)
	// since we failed, we should requeue the item to work on later.  This
	// method will add a backoff to avoid hotlooping on particular items
	// (they're probably still not going to work right away) and overall
	// controller protection (everything I've done is broken, this controller
	// needs to calm down or it can starve other useful work) cases.
	c.networkQueue.AddRateLimited(item)
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Node resource
// with the current status of the resource.
func (c *Controller) syncNetworkHandler(ctx context.Context, item WorkerItem) error {
	logger := klog.FromContext(ctx)
	// Get the Node resource with this namespace/name
	namespace, name := item.Key.Namespace, item.Key.Name

	switch item.EventType {
	case AddEvent:
		logger.Info("Add event", "namespace", namespace, "name", name, "resource type", "Network")
		current := item.NewObject.(*wireflowv1alpha1.Network)
		//if network has nodes, sync labels
		if len(current.Spec.Nodes) > 0 {
			for _, nodeName := range current.Spec.Nodes {
				node, err := c.nodesLister.Nodes(namespace).Get(nodeName)
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}

				_, err = c.SyncNodeNetworkLabels(item.Key.Name, node)
				if err != nil {
					return err
				}
			}
		}

	case UpdateEvent:
		oldNetwork, current := item.OldObject.(*wireflowv1alpha1.Network), item.NewObject.(*wireflowv1alpha1.Network)
		adds, removes := utils.Differences(oldNetwork.Spec.Nodes, current.Spec.Nodes)
		// update network need update node's labels
		if len(removes) > 0 {
			for _, nodeName := range removes {
				node, err := c.nodesLister.Nodes(namespace).Get(nodeName)
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}

				nodeCopy := node.DeepCopy()
				nodeCopy.Spec.Network = utils.RemoveStringFromSlice(nodeCopy.Spec.Network, current.Name)

				removedLables := fmt.Sprintf("wireflow.io/%s", current.Name)
				delete(nodeCopy.Labels, removedLables)
				if len(nodeCopy.Labels) == 0 {
					delete(nodeCopy.Labels, "wireflow.io/has-network")
				}

				nodeCopy.SetLabels(nodeCopy.Labels)
				_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
					ctx, nodeCopy, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				logger.Info(
					"Remove node network label & spec networks",
					"namespace", namespace,
					"name", nodeName,
				)
			}
		}

		if len(adds) > 0 {
			for _, nodeName := range adds {
				node, err := c.nodesLister.Nodes(namespace).Get(nodeName)
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}

				nodeCopy := node.DeepCopy()
				nodeCopy.Spec.Network = utils.RemoveStringFromSlice(nodeCopy.Spec.Network, current.Name)
				addedLabels := fmt.Sprintf("wireflow.io/%s", current.Name)
				if nodeCopy.Labels == nil {
					nodeCopy.Labels = make(map[string]string)
				}
				nodeCopy.Labels[addedLabels] = "true"
				nodeCopy.SetLabels(nodeCopy.Labels)

				// set network to node spec
				nodeCopy.Spec.Network = append(nodeCopy.Spec.Network, current.Name)

				_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
					ctx, nodeCopy, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				logger.Info(
					"Add node network label & spec networks",
					"namespace", namespace,
					"name", nodeName,
				)
			}
		}

	case DeleteEvent:
		//删除network资源后的处理
		labels := fmt.Sprintf("wireflow.io/%s", name)
		nodes, err := c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labels,
		})

		if err != nil {
			return err
		}

		for _, node := range nodes.Items {
			nodeCopy := node.DeepCopy()
			nodeCopy.Spec.Network = utils.RemoveStringFromSlice(nodeCopy.Spec.Network, name)

			if len(nodeCopy.Spec.Network) == 0 {
				nodeCopy.Spec.Address = ""
			}

			removedLables := fmt.Sprintf("wireflow.io/%s", name)
			delete(nodeCopy.Labels, removedLables)
			if len(nodeCopy.Labels) == 0 {
				delete(nodeCopy.Labels, "wireflow.io/has-network")
			}

			_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
				ctx, nodeCopy, metav1.UpdateOptions{})
			if err != nil {
			}
		}

		logger.Info("Delete event", "namespace", namespace, "name", name, "resource type", "Network")

	}

	return nil
}

// SyncNodeNetworkLabels syncs node labels based on networks the node belongs to
func (c *Controller) SyncNodeNetworkLabels(network string, node *wireflowv1alpha1.Node) (*wireflowv1alpha1.Node, error) {
	nodeCopy := node.DeepCopy()

	// Initialize labels map if nil
	if nodeCopy.Labels == nil {
		nodeCopy.Labels = make(map[string]string)
	}

	// Add new network labels
	labelKey := fmt.Sprintf("wireflow.io/%s", network)
	nodeCopy.Labels[labelKey] = "true"

	// Add network to node spec
	nodeCopy.Spec.Network = append(nodeCopy.Spec.Network, network)
	if len(node.Spec.Network) > 0 {
		// Also add a common label to identify all nodes with networks
		nodeCopy.Labels["wireflow.io/has-network"] = "true"
	} else {
		delete(nodeCopy.Labels, "wireflow.io/has-network")
	}

	// Update the node
	updatedNode, err := c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
		context.TODO(),
		nodeCopy,
		metav1.UpdateOptions{},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update node labels: %v", err)
	}

	return updatedNode, nil
}
