package resource

import (
	"context"
	"time"
	"wireflow/internal"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/controller"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type NetworkEventHandler struct {
	ctx      context.Context
	informer cache.SharedIndexInformer
	wt       *internal.WatchManager
	queue    workqueue.TypedRateLimitingInterface[controller.WorkerItem]
}

func (n *NetworkEventHandler) AddFunc(obj interface{})         {}
func (n *NetworkEventHandler) UpdateFunc(old, new interface{}) {}
func (n *NetworkEventHandler) DeleteFunc(obj interface{})      {}
func (n *NetworkEventHandler) EventType() EventType {
	return NetworkType
}

func NewNetworkEventHandler(ctx context.Context, informer cache.SharedIndexInformer, wt *internal.WatchManager, queue workqueue.TypedRateLimitingInterface[controller.WorkerItem]) *NetworkEventHandler {
	h := &NetworkEventHandler{
		ctx:      ctx,
		informer: informer,
		wt:       wt,
		queue:    queue,
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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
	return n.informer
}

func (n *NetworkEventHandler) RunWorker(ctx context.Context) {
	for n.ProcessNextItem() {
	}
}

func (n *NetworkEventHandler) ProcessNextItem() bool {
	return true
}

func (n *NetworkEventHandler) WorkQueue() workqueue.TypedRateLimitingInterface[controller.WorkerItem] {
	return n.queue
}
