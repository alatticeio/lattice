package resource

import (
	"context"
	"github.com/wireflowio/wireflow-controller/pkg/generated/informers/externalversions/wireflowcontroller/v1alpha1"
	"time"
	"wireflow/internal"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/controller"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type NodeEventHandler struct {
	ctx      context.Context
	informer v1alpha1.NodeInformer
	wt       *internal.WatchManager
	queue    workqueue.TypedRateLimitingInterface[controller.WorkerItem]
}

func NewNodeEventHandler(
	ctx context.Context,
	informer v1alpha1.NodeInformer,
	wt *internal.WatchManager,
	queue workqueue.TypedRateLimitingInterface[controller.WorkerItem]) *NodeEventHandler {
	h := &NodeEventHandler{
		ctx:      ctx,
		informer: informer,
		wt:       wt,
		queue:    queue,
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*wireflowv1alpha1.Node)
			if time.Since(node.CreationTimestamp.Time) > 5*time.Minute {
				klog.V(4).Infof("Skipping old node during initial sync: %s", node.Name)
				return
			}
			EnqueueItem(controller.AddEvent, nil, obj, h.queue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode, newNode := oldObj.(*wireflowv1alpha1.Node), newObj.(*wireflowv1alpha1.Node)
			if oldNode.ResourceVersion == newNode.ResourceVersion {
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

func (n *NodeEventHandler) EventType() EventType {
	return NodeType
}

func (n *NodeEventHandler) RunWorker(ctx context.Context) {
	for n.ProcessNextItem(ctx) {
	}
}

func (n *NodeEventHandler) syncHandler(ctx context.Context, item controller.WorkerItem) error {
	// Get the Node resource with this namespace/name
	namespace, name := item.Key.Namespace, item.Key.Name
	logger := klog.FromContext(ctx)

	switch item.EventType {
	case controller.AddEvent:
		logger.Info("Add event", "namespace", namespace, "name", name, "resource type", "Node")
	case controller.UpdateEvent:
		oldNode, newNode := item.OldObject.(*wireflowv1alpha1.Node), item.NewObject.(*wireflowv1alpha1.Node)
		logger.Info("Update event", "namespace", namespace, "name", name, "resource type", "Node")
		// ip 地址
		if oldNode.Spec.Address != newNode.Spec.Address {
			n.handleIPChange(newNode)
			logger.Info("Node IP address changed", "oldAddress", oldNode.Spec.Address, "newAddress", newNode.Spec.Address)
		}

	case controller.DeleteEvent:
		logger.Info("Delete event", "namespace", namespace, "name", name, "resource type", "Node")
	}

	return nil
}

func (n *NodeEventHandler) ProcessNextItem(ctx context.Context) bool {

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

func (n *NodeEventHandler) Informer() cache.SharedIndexInformer {
	return n.informer.Informer()
}

func (n *NodeEventHandler) WorkQueue() workqueue.TypedRateLimitingInterface[controller.WorkerItem] {
	return n.queue
}

// handleIPChange will send a new ip to client
func (n *NodeEventHandler) handleIPChange(node *wireflowv1alpha1.Node) {
	logger := klog.FromContext(context.Background())
	msg := new(internal.Message)
	msg.EventType = internal.EventTypeIPChange
	msg.Current = new(internal.Node)
	msg.Current.Address = node.Spec.Address
	msg.Current.AppID = node.Spec.AppId
	msg.Current.PublicKey = node.Spec.PublicKey
	msg.Current.PrivateKey = node.Spec.PrivateKey
	n.wt.Send(msg.Current.AppID, msg)
	logger.Info("Node IP address send to %s client success", "address", msg.Current.AppID, node.Spec.Address)
}

// handleNodeAdd will send msg to client
