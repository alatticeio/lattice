// Copyright 2025 The Wireflow Authors, Inc.
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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"wireflow/api/v1alpha1"

	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	cache2 "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type Client struct {
	client.Client
	manager.Manager

	log *log.Logger

	hashMu         sync.RWMutex
	lastPushedHash map[uint64]string
	sender         infra.SignalService
}

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

func NewClient(signal infra.SignalService, mgr manager.Manager) (*Client, error) {
	ctx := context.Background()
	logger := log.GetLogger("crd-client")

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

	client := &Client{
		Client:         mgr.GetClient(),
		lastPushedHash: make(map[uint64]string),
		log:            logger,
		sender:         signal,
		Manager:        mgr,
	}

	client.log.Info("Starting CRD Status Monitoring Agent...")
	// 2. 获取 Informer 并注册事件处理器
	informer, err := mgr.GetCache().GetInformer(ctx, &corev1.ConfigMap{})
	if err != nil {
		client.log.Error("failed to get informer for configMap", err)
		return nil, err
	}

	// 3. 注册事件回调函数
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logger.Info("Received add event for configMap", "obj", obj)
			client.handleConfigMapEvent(ctx, obj, "ADD")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// 默认 Informer 即使 RV 没变也会触发 Update。
			// 实际业务中，您可能需要比较新旧对象的 ResourceVersion 或 Status 字段来过滤。
			logger.Info("Received update event for configMap", "oldObj", oldObj, "newObj", newObj)
			client.handleConfigMapEvent(ctx, newObj, "UPDATE")
		},
		DeleteFunc: func(obj interface{}) {
			logger.Info("Received delete event for configMap", "obj", obj)
			client.handleConfigMapEvent(ctx, obj, "DELETE")
		},
	})
	return client, nil
}

// 核心事件处理函数
func (c *Client) handleConfigMapEvent(ctx context.Context, obj interface{}, eventType string) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		c.log.Info("Received object of unexpected type", "obj", obj)
		return
	}

	// 打印关键信息，包括 ResourceVersion 来追踪变化
	c.log.Info(">>> Event Detected <<<",
		"eventType", eventType,
		"namespace", cm.Namespace,
		"name", cm.Name,
		"version", cm.ResourceVersion,
	)

	var message infra.Message
	if err := json.Unmarshal([]byte(cm.Data["config.json"]), &message); err != nil {
		c.log.Error("Failed to unmarshal message", err)
		return
	}

	if message.Current != nil {
		c.pushToNode(ctx, message.Current.PeerID, &message)
		c.log.Info(">>> Message pushed to node success <<<", "namespace", cm.Namespace, "appId", message.Current.PublicKey, "version", cm.ResourceVersion)
	}
}

func (c *Client) pushToNode(ctx context.Context, peerId uint64, msg *infra.Message) error {
	// hash
	msgHash, err := c.computeMessageHash(msg)
	if err != nil {
		return err
	}

	// check hash
	c.hashMu.RLock()
	lastHash, exists := c.lastPushedHash[peerId]
	c.hashMu.RUnlock()

	if exists && lastHash == msgHash {
		c.log.Info("Message unchanged, skipping push", "peerId", peerId)
		return nil
	}

	// 3. 推送消息
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	packet := &grpc.SignalPacket{
		SenderId: peerId,
		Type:     grpc.PacketType_MESSAGE,
		Payload: &grpc.SignalPacket_Message{
			Message: &grpc.Message{
				Content: data,
			},
		},
	}

	content, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	if err := c.sender.Send(ctx, infra.FromUint64(peerId), content); err != nil {
		return fmt.Errorf("Failed to send message to node %s: %v", peerId, err)
	}

	// 4. 更新缓存
	c.hashMu.Lock()
	c.lastPushedHash[peerId] = msgHash
	c.hashMu.Unlock()

	// 5. 记录日志
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.log.Info("Pushing message success", "peerId", peerId, "data", len(b))
	return nil
}

func (c *Client) computeMessageHash(msg *infra.Message) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func NewManager() (manager.Manager, error) {
	// 1. 初始化 Manager (它是 Informer 和 Cache 的核心)
	// 默认会尝试加载集群内配置
	mgr, err := manager.New(ctrl.GetConfigOrDie(), manager.Options{
		Scheme: scheme,
		Cache: cache2.Options{
			DefaultLabelSelector: labels.SelectorFromSet(map[string]string{
				"app.kubernetes.io/managed-by": "wireflow-controller",
			}),
		},

		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})

	ctx := context.Background()
	// 注册索引： status.token
	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.WireflowEnrollmentToken{}, "status.token", func(rawObj client.Object) []string {
		// 1. 断言对象类型
		token, ok := rawObj.(*v1alpha1.WireflowEnrollmentToken)
		if !ok {
			return nil
		}
		// 2. 返回需要索引的字段值
		if token.Status.Token == "" {
			return nil
		}
		return []string{token.Status.Token}
	}); err != nil {
		return nil, err
	}

	// 只要你调用了 GetInformer，Manager 就会在 Start 时去同步它
	_, err = mgr.GetCache().GetInformer(ctx, &v1alpha1.WireflowEnrollmentToken{})
	if err != nil {
		return nil, fmt.Errorf("failed to start informer: %w", err)
	}
	return mgr, err
}
