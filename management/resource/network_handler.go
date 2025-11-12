package resource

import (
	"context"
	"crypto/sha3"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"wireflow/internal"
	utils2 "wireflow/pkg/utils"

	clientset "github.com/wireflowio/wireflow-controller/pkg/generated/clientset/versioned"
	"github.com/wireflowio/wireflow-controller/pkg/generated/informers/externalversions/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/controller"
	listers "github.com/wireflowio/wireflow-controller/pkg/generated/listers/wireflowcontroller/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type NetworkEventHandler struct {
	ctx        context.Context
	informer   v1alpha1.NetworkInformer
	clientSet  *clientset.Clientset
	nodeLister listers.NodeLister
	wt         *internal.WatchManager
	queue      workqueue.TypedRateLimitingInterface[controller.WorkerItem]
}

func (n *NetworkEventHandler) AddFunc(obj interface{})         {}
func (n *NetworkEventHandler) UpdateFunc(old, new interface{}) {}
func (n *NetworkEventHandler) DeleteFunc(obj interface{})      {}
func (n *NetworkEventHandler) EventType() EventType {
	return NetworkType
}

func NewNetworkEventHandler(
	ctx context.Context,
	informer v1alpha1.NetworkInformer,
	clientSet *clientset.Clientset,
	wt *internal.WatchManager,
	nodeLister listers.NodeLister,
	queue workqueue.TypedRateLimitingInterface[controller.WorkerItem]) *NetworkEventHandler {
	h := &NetworkEventHandler{
		ctx:        ctx,
		informer:   informer,
		wt:         wt,
		nodeLister: nodeLister,
		queue:      queue,
		clientSet:  clientSet,
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			network := obj.(*wireflowv1alpha1.Network)
			if time.Since(network.CreationTimestamp.Time) > 5*time.Minute {
				klog.V(4).Infof("Skipping old network during initial sync: %s", network.Name)
				return
			}
			EnqueueItem(controller.AddEvent, nil, obj, h.queue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNetwork, newNetwork := oldObj.(*wireflowv1alpha1.Network), newObj.(*wireflowv1alpha1.Network)
			if oldNetwork.ResourceVersion == newNetwork.ResourceVersion {
				return
			}
			EnqueueItem(controller.UpdateEvent, oldObj, newObj, h.queue)
		},
		DeleteFunc: func(obj interface{}) {
			EnqueueItem(controller.DeleteEvent, nil, obj, h.queue)
		},
	})

	return h
}

func (n *NetworkEventHandler) Informer() cache.SharedIndexInformer {
	return n.informer.Informer()
}

func (n *NetworkEventHandler) RunWorker(ctx context.Context) {
	for n.ProcessNextItem(ctx) {
	}
}

func (n *NetworkEventHandler) ProcessNextItem(ctx context.Context) bool {
	item, shutdown := n.queue.Get()
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
	defer n.queue.Done(item)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	err := n.syncHandler(ctx, item)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		n.queue.Forget(item)
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
	n.queue.AddRateLimited(item)

	return true
}

func (n *NetworkEventHandler) syncHandler(ctx context.Context, item controller.WorkerItem) error {
	// Get the Node resource with this namespace/name
	namespace, name := item.Key.Namespace, item.Key.Name
	logger := klog.FromContext(ctx)

	switch item.EventType {
	case controller.AddEvent:
		logger.Info("Add event", "namespace", namespace, "name", name, "resource type", "Network")
	case controller.UpdateEvent:
		oldNet, newNet := item.OldObject.(*wireflowv1alpha1.Network), item.NewObject.(*wireflowv1alpha1.Network)
		logger.Info("Update event", "namespace", namespace, "name", name, "old network", oldNet, "new network", newNet, "resource type", "Network")

		selector := labels.SelectorFromSet(labels.Set{fmt.Sprintf("wireflow.io/%s", newNet.Name): "true"})
		all, err := n.nodeLister.Nodes(namespace).List(selector)

		klog.Infof("all nodes: %v, err: %v", all, err)
		if err != nil {
			return err
		}

		adds, removes := utils.Differences(oldNet.Spec.Nodes, newNet.Spec.Nodes)
		// need seed to client to update node's dst nodes.
		if len(removes) > 0 {
			return n.handleNode(ctx, namespace, internal.EventTypeNodeRemove, all, removes)
		}

		if len(adds) > 0 {
			return n.handleNode(ctx, namespace, internal.EventTypeNodeAdd, all, adds)
		}

	case controller.DeleteEvent:
		logger.Info("Delete event", "namespace", namespace, "name", name, "resource type", "Network")
	}

	return nil
}

// handle node remove
func (n *NetworkEventHandler) handleNode(ctx context.Context, ns string, eventType internal.EventType, allNodes []*wireflowv1alpha1.Node, items []string) error {
	msg := &internal.Message{
		EventType: eventType,
		Current:   new(internal.Node),
		Network: &internal.Network{
			Nodes: make([]*internal.Node, 0),
		},
	}

	for _, name := range items {
		//processNode, err := n.clientSet.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Get(ctx, name, v1.GetOptions{})
		processNode, err := n.nodeLister.Nodes(ns).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to get node %s: %v", name, err)
		}

		// if node address is empty, retry later
		if eventType == internal.EventTypeNodeAdd && processNode.Spec.Address == "" {
			return fmt.Errorf("node %s address is empty, retry later for push", name)
		}

		Node := &internal.Node{
			Address:    processNode.Spec.Address,
			PrivateKey: processNode.Spec.PrivateKey,
			PublicKey:  processNode.Spec.PublicKey,
			AllowedIPs: fmt.Sprintf("%s/32", processNode.Spec.Address),
		}

		msg.Network.Nodes = append(msg.Network.Nodes, Node)
	}

	if len(msg.Network.Nodes) > 0 {
		for _, node := range allNodes {
			n.wt.Send(node.Spec.AppId, msg.WithNode(&internal.Node{
				PublicKey:  node.Spec.PublicKey,
				PrivateKey: node.Spec.PrivateKey,
				Address:    node.Spec.Address,
			}))
			b, _ := json.Marshal(msg)
			klog.Infof("send data to client: %v, data: %v", node.Spec.AppId, string(b))
		}
	}
	return nil
}

func (n *NetworkEventHandler) handleNetworkNodes(ctx context.Context, event string, handleNodes []string, namespace string) error {
	nodes := make([]*wireflowv1alpha1.Node, 0)
	allNodes := make([]*internal.Node, 0)
	for _, nodeName := range handleNodes {
		// remove node from old node list
		node, err := n.nodeLister.Nodes(namespace).Get(nodeName)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to get node %s: %v", nodeName, err)
		}
		handleNode := new(internal.Node)
		handleNode.Address = node.Spec.Address
		handleNode.PublicKey = node.Spec.PublicKey
		handleNode.PrivateKey = node.Spec.PrivateKey
		allNodes = append(allNodes, handleNode)
		nodes = append(nodes, node)
	}

	for _, node := range nodes {
		n.handelNodeType(ctx, event, node, allNodes)
	}
	return nil
}

func (n *NetworkEventHandler) WorkQueue() workqueue.TypedRateLimitingInterface[controller.WorkerItem] {
	return n.queue
}

func (n *NetworkEventHandler) handelNodeType(ctx context.Context, event string, node *wireflowv1alpha1.Node, dstNodes []*internal.Node) {
	logger := klog.FromContext(ctx)
	msg := new(internal.Message)
	switch event {
	case event:
	case "remove":
		msg.EventType = internal.EventTypeNodeRemove
	case "add":
		msg.EventType = internal.EventTypeNodeAdd
	}
	msg.Current = new(internal.Node)
	msg.Current.Address = node.Spec.Address
	msg.Current.PublicKey = node.Spec.PublicKey
	msg.Current.AppID = node.Spec.AppId
	msg.Current.PrivateKey = node.Spec.PrivateKey

	allowIps := make([]string, 0)
	for _, dstNode := range dstNodes {
		allowIps = append(allowIps, fmt.Sprintf("%s/32", dstNode.Address))
	}

	klog.Infof("handle node %s allowed ips %s", node.Spec.Address, allowIps)
	msg.Current.AllowedIPs = strings.Join(allowIps, ",")

	msg.Network = new(internal.Network)
	msg.Network.Nodes = append(msg.Network.Nodes, dstNodes...)
	n.wt.Send(msg.Current.AppID, msg)
	b, err := json.MarshalIndent(msg, "", " ")
	if err != nil {
		logger.Error(err, "marshal message error")
	}
	logger.Info("Send to client success", "address", node.Spec.Address, "data", string(b))
}

// handlePolicyHash
func (n *NetworkEventHandler) handlePolicyHash(ctx context.Context, event string, networkName string) (string, error) {

	objs, err := n.informer.Informer().GetIndexer().ByIndex("network", networkName)
	if err != nil {
		return "", err
	}

	policies := make([]*wireflowv1alpha1.NetworkPolicy, 0)
	for _, obj := range objs {
		if policy, ok := obj.(*wireflowv1alpha1.NetworkPolicy); ok {
			policies = append(policies, policy)
		}
	}

	//sort polices by priority

	// then hash it
	data, err := json.Marshal(policies)
	if err != nil {
		return "", err
	}
	return string(sha3.New256().Sum(data)), nil
}

// handlePolicy
func (n *NetworkEventHandler) handlePolicy(ctx context.Context, oldPolices []*wireflowv1alpha1.NetworkPolicy, newPolices []*wireflowv1alpha1.NetworkPolicy) error {
	m := make(map[string]*wireflowv1alpha1.NetworkPolicy)
	for _, op := range newPolices {
		m[op.Name] = op
	}

	for _, np := range newPolices {
		if _, ok := m[np.Name]; !ok {
			//not ok, is a new policy
			continue
		} else {
			newData, err := json.Marshal(np.Spec)
			if err != nil {
				return err
			}
			oldData, err := json.Marshal(m[np.Name].Spec)
			if err != nil {
				return err
			}

			// if ==, not changed
			if utils2.Hash(oldData) == utils2.Hash(newData) {
				continue
			} else {
				//重新推送策略
			}

		}
	}

	return nil
}
