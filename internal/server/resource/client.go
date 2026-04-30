// Copyright 2026 The Lattice Authors, Inc.
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
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/grpc"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/alatticeio/lattice/api/v1alpha1"

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
	lastPushedHash map[string]string
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
		lastPushedHash: make(map[string]string),
		log:            logger,
		sender:         signal,
		Manager:        mgr,
	}

	client.log.Info("CRD status monitor starting")
	// 2. 获取 Informer 并注册事件处理器
	informer, err := mgr.GetCache().GetInformer(ctx, &corev1.ConfigMap{})
	if err != nil {
		client.log.Error("failed to get informer for configMap", err)
		return nil, err
	}

	// 3. 注册事件回调函数
	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			client.handleConfigMapEvent(ctx, obj, "ADD")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			client.handleConfigMapEvent(ctx, newObj, "UPDATE")
		},
		DeleteFunc: func(obj interface{}) {
			client.handleConfigMapEvent(ctx, obj, "DELETE")
		},
	})

	if err != nil {
		return nil, err
	}
	return client, nil
}

// 核心事件处理函数
func (c *Client) handleConfigMapEvent(ctx context.Context, obj interface{}, eventType string) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		c.log.Warn("configmap event: unexpected object type", "type", fmt.Sprintf("%T", obj))
		return
	}

	c.log.Debug("configmap event",
		"type", eventType,
		"namespace", cm.Namespace,
		"name", cm.Name,
		"version", cm.ResourceVersion,
	)

	var message infra.Message
	if err := json.Unmarshal([]byte(cm.Data["config.json"]), &message); err != nil {
		c.log.Error("failed to unmarshal configmap config", err, "name", cm.Name, "namespace", cm.Namespace)
		return
	}

	if message.Current != nil {
		err := c.pushToNode(ctx, message.Current, &message)
		if err != nil {
			c.log.Error("failed to dispatch config to node", err, "app_id", message.Current.AppID)
			return
		}
		c.log.Info("config dispatched to node", "app_id", message.Current.AppID, "namespace", cm.Namespace, "version", cm.ResourceVersion)
	}
}

func (c *Client) pushToNode(ctx context.Context, peer *infra.Peer, msg *infra.Message) error {
	// hash
	msgHash, err := c.computeMessageHash(msg)
	if err != nil {
		return err
	}

	// check hash by appId (stable across key rotations)
	c.hashMu.RLock()
	lastHash, exists := c.lastPushedHash[peer.AppID]
	c.hashMu.RUnlock()

	if exists && lastHash == msgHash {
		c.log.Debug("config unchanged, skipping dispatch", "app_id", peer.AppID)
		return nil
	}

	// derive PeerID from public key for NATS routing
	pubKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key for peer %s: %v", peer.AppID, err)
	}
	peerID := infra.FromKey(pubKey)

	// push message
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	packet := &grpc.SignalPacket{
		SenderId: peerID.ToUint64(),
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

	if err = c.sender.Send(ctx, peerID, content); err != nil {
		return fmt.Errorf("failed to send message to node %s: %v", peer.AppID, err)
	}

	// update cache
	c.hashMu.Lock()
	c.lastPushedHash[peer.AppID] = msgHash
	c.hashMu.Unlock()

	c.log.Debug("config dispatch acknowledged", "app_id", peer.AppID, "payload_bytes", len(data))
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
	// 使用 GetConfig() 替代 GetConfigOrDie()，避免非 K8s 环境下进程直接 exit。
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig (not in K8s cluster?): %w", err)
	}

	mgr, err := manager.New(restConfig, manager.Options{
		Scheme: scheme,
		Cache: cache2.Options{
			// 只对 ConfigMap 限定 label，避免 informer 缓存所有 ConfigMap。
			// 其他 CRD 类型不做过滤，走完整缓存，对 API server 压力最小。
			ByObject: map[client.Object]cache2.ByObject{
				&corev1.ConfigMap{}: {
					Label: labels.SelectorFromSet(map[string]string{
						"app.kubernetes.io/managed-by": "lattice-controller",
					}),
				},
			},
		},

		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})

	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	// 注册索引： status.token
	if err = mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.LatticeEnrollmentToken{}, "status.token", func(rawObj client.Object) []string {
		token, ok := rawObj.(*v1alpha1.LatticeEnrollmentToken)
		if !ok {
			return nil
		}
		if token.Status.Token == "" {
			return nil
		}
		return []string{token.Status.Token}
	}); err != nil {
		return nil, err
	}

	// 注册索引： spec.token（兼容旧逻辑）
	if err = mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.LatticeEnrollmentToken{}, "spec.token", func(rawObj client.Object) []string {
		// 1. 断言对象类型
		token, ok := rawObj.(*v1alpha1.LatticeEnrollmentToken)
		if !ok {
			return nil
		}
		// 2. 返回需要索引的字段值
		if token.Spec.Token == "" {
			return nil
		}
		return []string{token.Spec.Token}
	}); err != nil {
		return nil, err
	}

	// 只要你调用了 GetInformer，Manager 就会在 Start 时去同步它
	_, err = mgr.GetCache().GetInformer(ctx, &v1alpha1.LatticeEnrollmentToken{})
	if err != nil {
		return nil, fmt.Errorf("failed to start informer: %w", err)
	}
	return mgr, err
}
