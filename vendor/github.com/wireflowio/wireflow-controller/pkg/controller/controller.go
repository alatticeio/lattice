package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/wireflowio/wireflow-controller/pkg/http"
	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	clientset "github.com/wireflowio/wireflow-controller/pkg/generated/clientset/versioned"
	samplescheme "github.com/wireflowio/wireflow-controller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/wireflowio/wireflow-controller/pkg/generated/informers/externalversions/wireflowcontroller/v1alpha1"
	listers "github.com/wireflowio/wireflow-controller/pkg/generated/listers/wireflowcontroller/v1alpha1"
)

const controllerAgentName = "wireflow-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Node is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Node fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Node"
	// MessageResourceSynced is the message used for an Event fired when a Node
	// is synced successfully
	MessageResourceSynced = "Node synced successfully"
	// FieldManager distinguishes this controller from other things writing to API objects
	FieldManager = controllerAgentName
)

// Controller is the controller implementation for Node resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// wireflowclientset is a clientset for our own API group
	wireflowclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced

	nodeInformer informers.NodeInformer
	nodesLister  listers.NodeLister
	nodesSynced  cache.InformerSynced

	networkInformer informers.NetworkInformer
	networkLister   listers.NetworkLister
	networkSynced   cache.InformerSynced

	networkPolicyLister listers.NetworkPolicyLister
	networkPolicySynced cache.InformerSynced

	httpClient *http.HttpClient

	// nodeQueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	nodeQueue          workqueue.TypedRateLimitingInterface[WorkerItem]
	networkQueue       workqueue.TypedRateLimitingInterface[WorkerItem]
	networkPolicyQueue workqueue.TypedRateLimitingInterface[WorkerItem]
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	ipAllocator *IPAllocator

	policyChangeDetector *PolicyChangeDetector
}

type EventType string

var (
	AddEvent    EventType = "ADD"
	UpdateEvent EventType = "UPDATE"
	DeleteEvent EventType = "DELETE"
)

type WorkerItem struct {
	Key       cache.ObjectName
	EventType EventType
	OldObject interface{}
	NewObject interface{}
}

// NewController returns a new sample controller
func NewController(
	ctx context.Context,
	kubeclientset kubernetes.Interface,
	wireflowclientset clientset.Interface,
	httpClient *http.HttpClient,
	nodeInformer informers.NodeInformer,
	networkInformer informers.NetworkInformer,
	networkPolicyInformer informers.NetworkPolicyInformer) *Controller {
	logger := klog.FromContext(ctx)

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[WorkerItem](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[WorkerItem]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &Controller{
		kubeclientset:        kubeclientset,
		wireflowclientset:    wireflowclientset,
		httpClient:           httpClient,
		nodesLister:          nodeInformer.Lister(),
		networkLister:        networkInformer.Lister(),
		nodeInformer:         nodeInformer,
		nodesSynced:          nodeInformer.Informer().HasSynced,
		networkSynced:        networkInformer.Informer().HasSynced,
		networkPolicySynced:  networkPolicyInformer.Informer().HasSynced,
		nodeQueue:            workqueue.NewTypedRateLimitingQueue(ratelimiter),
		networkInformer:      networkInformer,
		networkQueue:         workqueue.NewTypedRateLimitingQueue(ratelimiter),
		networkPolicyQueue:   workqueue.NewTypedRateLimitingQueue(ratelimiter),
		recorder:             recorder,
		ipAllocator:          NewIPAllocator(),
		policyChangeDetector: NewPolicyChangeDetector(),
		networkPolicyLister:  networkPolicyInformer.Lister(),
	}

	logger.Info("Setting up event handlers")
	// Set up an event handler for when Node resources change
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*wireflowv1alpha1.Node)
			if time.Since(node.CreationTimestamp.Time) > 5*time.Minute {
				klog.V(4).Infof("Skipping old node during initial sync: %s", node.Name)
				return
			}
			//加入队列
			controller.enqueue(AddEvent, obj, nil, controller.nodeQueue)
		},
		UpdateFunc: func(old, new interface{}) {
			oldNode, newNode := old.(*wireflowv1alpha1.Node), new.(*wireflowv1alpha1.Node)
			if oldNode.ResourceVersion == newNode.ResourceVersion {
				return
			}
			controller.enqueue(UpdateEvent, old, new, controller.nodeQueue)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueue(DeleteEvent, obj, nil, controller.nodeQueue)
		},
	})

	networkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			network := obj.(*wireflowv1alpha1.Network)
			if time.Since(network.CreationTimestamp.Time) > 5*time.Minute {
				klog.V(4).Infof("Skipping old node during initial sync: %s", network.Name)
				return
			}
			//加入队列
			controller.enqueue(AddEvent, nil, obj, controller.networkQueue)
		},
		UpdateFunc: func(old, new interface{}) {
			oldNetwork, newNetwork := old.(*wireflowv1alpha1.Network), new.(*wireflowv1alpha1.Network)
			if oldNetwork.ResourceVersion == newNetwork.ResourceVersion {
				return
			}
			controller.enqueue(UpdateEvent, old, new, controller.networkQueue)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueue(DeleteEvent, obj, nil, controller.networkQueue)
		},
	})

	//通过netwrokName索引所有的Node
	nodeInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		"network": func(obj interface{}) ([]string, error) {
			node := obj.(*wireflowv1alpha1.Node)
			return node.Spec.Network, nil
		},
	})

	//通过策略名称索引所有的Network
	networkInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		"policy": func(obj interface{}) ([]string, error) {
			network := obj.(*wireflowv1alpha1.Network)
			return network.Spec.Polices, nil
		},
	})

	networkPolicyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			networkPolicy := obj.(*wireflowv1alpha1.NetworkPolicy)
			if time.Since(networkPolicy.CreationTimestamp.Time) > 5*time.Minute {
				klog.V(4).Infof("Skipping old node during initial sync: %s", networkPolicy.Name)
				return
			}
			//加入队列
			controller.enqueue(AddEvent, nil, obj, controller.networkPolicyQueue)
		},
		UpdateFunc: func(old, new interface{}) {
			oldPolicy, newPolicy := old.(*wireflowv1alpha1.NetworkPolicy), new.(*wireflowv1alpha1.NetworkPolicy)
			if oldPolicy.ResourceVersion == newPolicy.ResourceVersion {
				return
			}
			controller.enqueue(UpdateEvent, old, new, controller.networkPolicyQueue)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueue(DeleteEvent, obj, nil, controller.networkPolicyQueue)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the nodeQueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.nodeQueue.ShutDown()
	logger := klog.FromContext(ctx)

	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting Node controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")

	// node synced
	if ok := cache.WaitForCacheSync(ctx.Done(), c.nodesSynced, c.networkSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Node resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runNodeWorker, time.Second)
		go wait.UntilWithContext(ctx, c.runNetworkWorker, time.Second)
		go wait.UntilWithContext(ctx, c.runNetworkPolicyWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

// enqueueNode takes a Node resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Node.
func (c *Controller) enqueue(eventType, old, new interface{}, queue workqueue.TypedRateLimitingInterface[WorkerItem]) {
	var (
		objectRef cache.ObjectName
		err       error
	)
	if old != nil {
		objectRef, err = cache.ObjectToName(old)
	} else if new != nil {
		objectRef, err = cache.ObjectToName(new)
	}

	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	item := WorkerItem{}
	item.Key = objectRef
	switch eventType {
	case AddEvent:
		item.EventType = AddEvent
		item.NewObject = new
	case DeleteEvent:
		item.EventType = DeleteEvent
		item.OldObject = old
	case UpdateEvent:
		item.EventType = UpdateEvent
		item.OldObject = old
		item.NewObject = new
	}

	queue.Add(item)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Node resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Node resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	//var object metav1.Object
	//var ok bool
	//logger := klog.FromContext(context.Background())
	//if object, ok = obj.(metav1.Object); !ok {
	//	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	//	if !ok {
	//		// If the object value is not too big and does not contain sensitive information then
	//		// it may be useful to include it.
	//		utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object, invalid type", "type", fmt.Sprintf("%T", obj))
	//		return
	//	}
	//	object, ok = tombstone.Obj.(metav1.Object)
	//	if !ok {
	//		// If the object value is not too big and does not contain sensitive information then
	//		// it may be useful to include it.
	//		utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object tombstone, invalid type", "type", fmt.Sprintf("%T", tombstone.Obj))
	//		return
	//	}
	//	logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
	//}
	//logger.V(4).Info("Processing object", "object", klog.KObj(object))
	//if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
	//	// If this object is not owned by a Node, we should not do anything more
	//	// with it.
	//	if ownerRef.Kind != "Node" {
	//		return
	//	}
	//
	//	node, err := c.nodesLister.Nodes(object.GetNamespace()).Get(ownerRef.Name)
	//	if err != nil {
	//		logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "node", ownerRef.Name)
	//		return
	//	}
	//
	//	c.enqueue(node)
	//	return
	//}
}

// GetNetworkByPolicyName 获取指定策略所关联的网络
func (c *Controller) GetNetworkByPolicyName(policyName string) ([]*wireflowv1alpha1.Network, error) {
	objs, err := c.networkInformer.Informer().GetIndexer().ByIndex("networkPolicyName", policyName)
	if err != nil {
		return nil, err
	}

	ans := make([]*wireflowv1alpha1.Network, 0)
	for _, obj := range objs {
		if net, ok := obj.(*wireflowv1alpha1.Network); ok {
			ans = append(ans, net)
		}
	}

	return ans, nil
}
