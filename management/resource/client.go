package resource

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"wireflow/internal"
	"wireflow/pkg/log"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	cache2 "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Client struct {
	client  client.Client
	manager manager.Manager

	log *log.Logger

	hashMu         sync.RWMutex
	lastPushedHash map[string]string
	wt             *internal.WatchManager
}

var scheme = runtime.NewScheme()

func init() {
	// 注册 Kubernetes 内置资源 Scheme（例如 Pod, Deployment）
	_ = clientgoscheme.AddToScheme(scheme)

	// 🚨 注册你的 CRD Scheme（必须！）
	// 这使得 client.Client 知道如何序列化和反序列化你的 Network 资源
	_ = wireflowv1alpha1.AddToScheme(scheme)

	// 如果有其他自定义资源，也需在此注册
}

func NewClient(wt *internal.WatchManager) (*Client, error) {
	ctx := context.Background()
	logger := log.NewLogger(log.Loglevel, "crdclient")

	// 1. Define Zap Options
	// By default, it uses Production JSON format.
	// opts.Development = true provides a more human-readable text output (recommended for local development).
	opts := zap.Options{
		Development: true,
		// DisableStacktrace: true, // You may want to disable stack traces for cleaner logs
	}

	// 2. Initialize the log using the options
	zapLogger := zap.New(zap.UseFlagOptions(&opts))

	// 3. Set the initialized log for controller-runtime
	logf.SetLogger(zapLogger)

	// 2. 获取 Kubernetes 配置
	config, err := loadKubeConfig()
	if err != nil {
		return nil, err
	}

	// 3. 创建 client-runtime 的通用 Client
	crdClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		logger.Errorf("Error creating client: %v", err)
	}

	client := &Client{
		client:         crdClient,
		lastPushedHash: make(map[string]string),
		wt:             wt,
		log:            logger,
	}
	// 1. 初始化 Manager (它是 Informer 和 Cache 的核心)
	// 默认会尝试加载集群内配置
	mgr, err := manager.New(ctrl.GetConfigOrDie(), manager.Options{
		Scheme: scheme,
		Cache: cache2.Options{
			DefaultLabelSelector: labels.SelectorFromSet(map[string]string{
				"app.kubernetes.io/managed-by": "wireflow-controller",
			}),
		},
	})
	if err != nil {
		client.log.Errorf("unable to start manager: %v", err)
	}

	client.manager = mgr

	client.log.Infof("Starting CRD Status Monitoring Agent...")
	// 2. 获取 Informer 并注册事件处理器
	informer, err := mgr.GetCache().GetInformer(ctx, &corev1.ConfigMap{})
	if err != nil {
		client.log.Errorf("failed to get informer for configMap: %v", err)
	}

	// 3. 注册事件回调函数
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			client.handleConfigMapEvent(ctx, obj, "ADD")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// 默认 Informer 即使 RV 没变也会触发 Update。
			// 实际业务中，您可能需要比较新旧对象的 ResourceVersion 或 Status 字段来过滤。
			client.handleConfigMapEvent(ctx, newObj, "UPDATE")
		},
		DeleteFunc: func(obj interface{}) {
			client.handleConfigMapEvent(ctx, obj, "DELETE")
		},
	})
	return client, nil
}

func (c *Client) Start() error {
	var err error
	// 3. 启动 Manager (这将启动所有的 Informer 和缓存)
	if err = c.manager.Start(context.Background()); err != nil {
		c.log.Errorf("problem running manager: %v", err)
		return err
	}

	return nil
}

// loadKubeConfig 尝试加载集群内配置或本地 kubeconfig
func loadKubeConfig() (*rest.Config, error) {
	// 尝试加载集群内配置（如果在 Pod 中运行）
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// 尝试加载本地 kubeconfig
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig file not found at %s", kubeconfig)
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// 核心事件处理函数
func (c *Client) handleConfigMapEvent(ctx context.Context, obj interface{}, eventType string) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		c.log.Infof("Received object of unexpected type: %T", obj)
		return
	}

	// 打印关键信息，包括 ResourceVersion 来追踪变化
	c.log.Infof(">>> [%s] Event Detected <<< Name: %s/%s, RV: %s",
		eventType,
		cm.Namespace,
		cm.Name,
		cm.ResourceVersion,
	)

	// 可以在这里添加您的自定义业务逻辑，例如触发配置推送

	var message internal.Message
	if err := json.Unmarshal([]byte(cm.Data["config.json"]), &message); err != nil {
		c.log.Errorf("Failed to unmarshal message: %v", err)
	}

	c.pushToNode(ctx, message.Current.AppID, &message)
	c.log.Infof(">>> Message pushed to node success <<< Name: %s/%s, RV: %s", cm.Namespace, message.Current.AppID, cm.ResourceVersion)
}

func (c *Client) pushToNode(ctx context.Context, appId string, msg *internal.Message) error {
	// 1. 计算消息哈希
	msgHash := c.computeMessageHash(msg)

	// 2. 检查是否与上次推送相同
	c.hashMu.RLock()
	lastHash, exists := c.lastPushedHash[appId]
	c.hashMu.RUnlock()

	if exists && lastHash == msgHash {
		c.log.Infof("Message unchanged, skipping push appId: %v", appId)
		return nil
	}

	// 3. 推送消息
	if err := c.wt.Send(appId, msg); err != nil {
		return fmt.Errorf("failed to send message to node %s: %v", appId, err)
	}

	// 4. 更新缓存
	c.hashMu.Lock()
	c.lastPushedHash[appId] = msgHash
	c.hashMu.Unlock()

	// 5. 记录日志
	b, _ := json.Marshal(msg)
	c.log.Infof(">>> Pushed message to node: %s, eventType: %s, size: %v", appId, "ConfigUpdate", len(b))
	return nil
}

func (c *Client) computeMessageHash(msg *internal.Message) string {
	data, _ := json.Marshal(msg)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
