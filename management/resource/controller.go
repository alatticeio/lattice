package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wireflow/internal"

	"github.com/wireflowio/wireflow-controller/pkg/controller"
	clientset "github.com/wireflowio/wireflow-controller/pkg/generated/clientset/versioned"
	informers "github.com/wireflowio/wireflow-controller/pkg/generated/informers/externalversions"
	"golang.org/x/time/rate"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type Controller struct {
	wt              *internal.WatchManager
	Clientset       *clientset.Clientset
	EventHandlers   []EventHandler
	nodeQueue       workqueue.TypedRateLimitingInterface[controller.WorkerItem]
	networkQueue    workqueue.TypedRateLimitingInterface[controller.WorkerItem]
	policyQueue     workqueue.TypedRateLimitingInterface[controller.WorkerItem]
	ruleQueue       workqueue.TypedRateLimitingInterface[controller.WorkerItem]
	informerFactory informers.SharedInformerFactory
	nodeSynced      cache.InformerSynced
}

func NewController(
	ctx context.Context,
	kubeconfig string,
	wt *internal.WatchManager,
) (*Controller, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// 使用默认的 kubeconfig 路径
		if home, _ := os.UserHomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	// 尝试使用 kubeconfig 文件
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Warningf("using in-cluster configuration: %v", err)
		// 如果失败，尝试使用 in-cluster 配置
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("无法创建 kubernetes 配置: %v", err)
		}
	}

	// 创建 Clientset
	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("无法创建 Clientset: %v", err)
	}

	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[controller.WorkerItem](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[controller.WorkerItem]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	informerFactory := informers.NewSharedInformerFactory(cs, time.Minute*10)

	nodeInformer := informerFactory.Wireflowcontroller().V1alpha1().Nodes().Informer()
	//networkInformer := informerFactory.Wireflowcontroller().V1alpha1().Networks().Informer()

	nodeQueue, networkQueue := workqueue.NewTypedRateLimitingQueue(ratelimiter), workqueue.NewTypedRateLimitingQueue(ratelimiter)
	eventHandlers := make([]EventHandler, 0)
	eventHandlers = append(eventHandlers,
		NewNodeEventHandler(ctx, nodeInformer, wt, nodeQueue),
	)

	c := &Controller{
		Clientset:       cs,
		EventHandlers:   eventHandlers,
		wt:              wt,
		nodeQueue:       nodeQueue,
		networkQueue:    networkQueue,
		informerFactory: informerFactory,
		nodeSynced:      nodeInformer.HasSynced,
	}

	informerFactory.Start(ctx.Done())

	return c, nil
}

func (c *Controller) Run(ctx context.Context) error {
	syncList := make([]cache.InformerSynced, 0)
	if c.EventHandlers != nil {
		for _, handler := range c.EventHandlers {
			go wait.UntilWithContext(ctx, handler.RunWorker, time.Second)
			syncList = append(syncList, handler.Informer().HasSynced)
		}
	}
	//等待缓存完成
	if !cache.WaitForCacheSync(ctx.Done(), c.nodeSynced) {
		klog.Errorf("cache sync failed")
	}
	klog.Info("resource cache sync successfully...")
	return nil
}

// enqueueNode takes a Node resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Node.
func EnqueueItem(eventType, old, new interface{}, queue workqueue.TypedRateLimitingInterface[controller.WorkerItem]) {
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
	item := controller.WorkerItem{}
	item.Key = objectRef
	switch eventType {
	case controller.AddEvent:
		item.EventType = controller.AddEvent
		item.NewObject = new
	case controller.DeleteEvent:
		item.EventType = controller.DeleteEvent
		item.OldObject = old
	case controller.UpdateEvent:
		item.EventType = controller.UpdateEvent
		item.OldObject = old
		item.NewObject = new
	}

	queue.Add(item)
}
