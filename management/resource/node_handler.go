package resource

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"wireflow/internal"

	"github.com/wireflowio/wireflow-controller/pkg/generated/informers/externalversions/wireflowcontroller/v1alpha1"
	listers "github.com/wireflowio/wireflow-controller/pkg/generated/listers/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/utils"
	"k8s.io/apimachinery/pkg/labels"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"github.com/wireflowio/wireflow-controller/pkg/controller"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type NodeEventHandler struct {
	ctx            context.Context
	informer       v1alpha1.NodeInformer
	wt             *internal.WatchManager
	queue          workqueue.TypedRateLimitingInterface[controller.WorkerItem]
	lastPushedHash map[string]string
	hashMu         sync.RWMutex
	nodeLister     listers.NodeLister
	networkLister  listers.NetworkLister
	policyLister   listers.NetworkPolicyLister

	nodeContext map[string]*NodeContext
	contextMu   sync.RWMutex

	versionCounter uint64
	versionMu      sync.Mutex

	changeDetector *ChangeDetector
}

type StateCache struct {
	states map[string]string
	sync.RWMutex
}

func NewNodeEventHandler(
	ctx context.Context,
	informer v1alpha1.NodeInformer,
	wt *internal.WatchManager,
	networkLister listers.NetworkLister,
	policyLister listers.NetworkPolicyLister,
	queue workqueue.TypedRateLimitingInterface[controller.WorkerItem]) *NodeEventHandler {
	h := &NodeEventHandler{
		ctx:            ctx,
		informer:       informer,
		wt:             wt,
		queue:          queue,
		lastPushedHash: make(map[string]string),
		changeDetector: NewChangeDetector(),
		networkLister:  networkLister,
		policyLister:   policyLister,
		nodeContext:    make(map[string]*NodeContext),
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

			// ready的时候才会推送
			if newNode.Status.Phase == wireflowv1alpha1.NodeReady {
				EnqueueItem(controller.UpdateEvent, oldObj, newObj, h.queue)
			}
		},
		DeleteFunc: func(obj interface{}) {
			EnqueueItem(controller.DeleteEvent, nil, obj, h.queue)

			h.contextMu.Lock()
			delete(h.nodeContext, obj.(*wireflowv1alpha1.Node).Spec.AppId)
			h.contextMu.Unlock()

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
		// 新节点加入
		node := item.NewObject.(*wireflowv1alpha1.Node)
		logger.Info("Node Add event", "namespace", namespace, "name", name, "appId", node.Spec.AppId)

		// 检查节点状态，决定是否推送
		return n.reconcileNodeAdd(ctx, node)

	case controller.UpdateEvent:
		oldNode := item.OldObject.(*wireflowv1alpha1.Node)
		newNode := item.NewObject.(*wireflowv1alpha1.Node)
		logger.Info("Node Update event", "namespace", namespace, "name", name, "appId", newNode.Spec.AppId)

		// 分析变化类型，执行相应的推送
		return n.reconcileNodeUpdate(ctx, oldNode, newNode)

	case controller.DeleteEvent:
		// 节点删除
		node := item.NewObject.(*wireflowv1alpha1.Node)
		logger.Info("Node Delete event", "namespace", namespace, "name", name, "appId", node.Spec.AppId)

		return n.reconcileNodeDelete(ctx, node)
	}

	return nil
}

// reconcileNodeAdd 处理节点新增
func (n *NodeEventHandler) reconcileNodeAdd(ctx context.Context, node *wireflowv1alpha1.Node) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node Add event", "node", node)

	// Node is not ready yet, skip
	if node.Status.Phase != wireflowv1alpha1.NodeReady {
		logger.Info("Node not ready, skip")
		return nil
	}

	if len(node.Spec.Network) == 0 {
		logger.Info("Node has no network, skip")
		return nil
	}

	msg := n.buildNodeConfig(ctx, node)

	// 5. 推送配置到节点
	return n.pushToNode(ctx, node, msg)
}

func (n *NodeEventHandler) buildNodeConfig(ctx context.Context, node *wireflowv1alpha1.Node) *internal.Message {
	logger := klog.FromContext(ctx)
	logger.Info("Build node config", "node", node)

	return &internal.Message{
		EventType: internal.EventTypeIPChange,
		Current: &internal.Node{
			Address:    node.Spec.Address,
			AppID:      node.Spec.AppId,
			PublicKey:  node.Spec.PublicKey,
			PrivateKey: node.Spec.PrivateKey,
		},
		Network: &internal.Network{
			Nodes: make([]*internal.Node, 0),
		},
	}
}

func (n *NodeEventHandler) buildMessage(ctx context.Context, event internal.EventType, node *internal.Node, network *internal.Network) *internal.Message {
	return &internal.Message{
		EventType: event,
		Current:   node,
		Network:   network,
	}
}

func (n *NodeEventHandler) initNode(ctx context.Context, node *wireflowv1alpha1.Node) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node init event", "node", node)

	return nil
}

func (n *NodeEventHandler) reconcileNodeUpdate(ctx context.Context, oldNode, newNode *wireflowv1alpha1.Node) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node Update event", "oldNode", oldNode, "newNode", newNode)

	//if !n.IsConnected(ctx, newNode) {
	//	logger.Info("Node not connected, skip")
	//	context := n.getNodeContext(newNode)
	//	n.cacheNodeContext(newNode.Name, context)
	//	return nil
	//}

	oldContext := n.getCachedNodeContext(oldNode)
	if oldContext == nil {
		oldContext = n.getNodeContext(oldNode)
		n.cacheNodeContext(oldNode.Name, oldContext)
	}
	newContext := n.getNodeContext(newNode)

	//更新缓存
	n.cacheNodeContext(newNode.Name, newContext)

	changes := n.changeDetector.DetectNodeChanges(oldNode, newNode, oldContext.Network, newContext.Network, oldContext.Policies, newContext.Policies)

	if !changes.HasChanges() {
		logger.V(4).Info("No significant changes tectected", "node", newNode.Name)
		return nil
	}

	msg, err := n.buildFullConfigurationWithChanges(newNode, newContext, changes)
	if err != nil {
		logger.Error(err, "Failed to build full config with changes", "node", newNode.Name)
		return err
	}

	logger.Info("Push full config with changes", "node", newNode.Name, "changes", changes.Summary(), "version", msg.ConfigVersion)

	return n.pushToNode(ctx, newNode, msg)
}

func (n *NodeEventHandler) getCachedNodeContext(node *wireflowv1alpha1.Node) *NodeContext {
	n.contextMu.RLock()
	defer n.contextMu.RUnlock()

	return n.nodeContext[node.Spec.AppId]
}

func (n *NodeEventHandler) cacheNodeContext(appId string, context *NodeContext) {
	n.contextMu.Lock()
	defer n.contextMu.Unlock()

	//DeepCopy避免引用问题
	cachedContext := &NodeContext{
		Node:     context.Node.DeepCopy(),
		Nodes:    make([]*wireflowv1alpha1.Node, len(context.Nodes)),
		Policies: make([]*wireflowv1alpha1.NetworkPolicy, len(context.Policies)),
	}

	if context.Network != nil {
		cachedContext.Network = context.Network.DeepCopy()
	}

	for i, node := range context.Nodes {
		cachedContext.Nodes[i] = node.DeepCopy()
	}

	for i, policy := range context.Policies {
		cachedContext.Policies[i] = policy.DeepCopy()
	}

	n.nodeContext[appId] = cachedContext
}

func (n *NodeEventHandler) analyzeEvent(oldNode, newNode *wireflowv1alpha1.Node) (internal.EventType, error) {
	logger := klog.FromContext(context.Background())
	logger.Info("Node changed", "node", newNode.Name, "status", newNode.Status.Status)

	//新节点
	if oldNode == nil {
		logger.V(4).Info("Node is new, skip push", "appId", newNode.Spec.AppId, "status", newNode.Status.Status)
		return internal.EventTypeNone, nil
	}

	//节点还未reconcile就绪
	if newNode.Status.Phase != wireflowv1alpha1.NodeReady {
		logger.V(4).Info("Node not ready, skip push", "appId", newNode.Spec.AppId, "status", newNode.Status.Status)
		return internal.EventTypeNone, nil
	}

	//节点不在线
	if newNode.Status.Status != wireflowv1alpha1.Active {
		logger.V(4).Info("Node not active, skip push", "appId", newNode.Spec.AppId, "status", newNode.Status.Status)
		return internal.EventTypeNone, nil
	}

	// node private key changed
	if n.SpecEquals(oldNode, newNode) {
		logger.V(4).Info("Node unchanged, skip push", "appId", newNode.Spec.AppId)
		return internal.EventTypeNodeUpdate, nil
	}

	if len(oldNode.Spec.Network) != len(newNode.Spec.Network) {
		logger.V(4).Info("Node network changed", "old", oldNode.Spec.Network, "new", newNode.Spec.Network)
		return internal.EventTypeNetworkChanged, nil
	}

	//网络配置了， IP地址变化
	if oldNode.Spec.Address != newNode.Spec.Address {
		logger.V(4).Info("Node address changed", "old", oldNode.Spec.Address, "new", newNode.Spec.Address)
		return internal.EventTypeIPChange, nil
	}

	// 主network 变化了
	if oldNode.Spec.Network[0] != newNode.Spec.Network[0] {
		logger.V(4).Info("Node network changed", "old", oldNode.Spec.Network, "new", newNode.Spec.Network)
		return internal.EventTypeNetworkChanged, nil
	}

	//策略变化
	if oldNode.Status.Phase == wireflowv1alpha1.NodeUpdatingPolicy && newNode.Status.Phase == wireflowv1alpha1.NodeReady {
		logger.V(4).Info("Node policy changed", "old", oldNode.Status.Phase, "new", newNode.Status.Phase)
		return internal.EventTypePolicyChanged, nil
	}

	adds, removes := utils.Differences(newNode.Spec.Network, oldNode.Spec.Network)
	if adds == nil && removes == nil {
		logger.V(4).Info("Node network unchanged", "adds", adds, "removes", removes)
		return internal.EventTypeNone, nil
	} else {
		logger.V(4).Info("Node network changed", "adds", adds, "removes", removes)
		return internal.EventTypeNodeUpdate, nil
	}

	return internal.EventTypeNone, nil
}

func (n *NodeEventHandler) reconcileNodeDelete(ctx context.Context, node *wireflowv1alpha1.Node) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node Delete event", "node", node)
	n.handleNodeDelete(ctx, node)
	return nil
}

// pushToNode 推送消息到节点（带去重检查）
func (h *NodeEventHandler) pushToNode(ctx context.Context, node *wireflowv1alpha1.Node, msg *internal.Message) error {
	logger := klog.FromContext(ctx)

	// 1. 计算消息哈希
	msgHash := h.computeMessageHash(msg)

	// 2. 检查是否与上次推送相同
	h.hashMu.RLock()
	lastHash, exists := h.lastPushedHash[node.Spec.AppId]
	h.hashMu.RUnlock()

	if exists && lastHash == msgHash {
		logger.V(4).Info("Message unchanged, skipping push", "appId", node.Spec.AppId)
		return nil
	}

	// 3. 推送消息
	if err := h.wt.Send(node.Spec.AppId, msg); err != nil {
		return fmt.Errorf("failed to send message to node %s: %v", node.Spec.AppId, err)
	}

	// 4. 更新缓存
	h.hashMu.Lock()
	h.lastPushedHash[node.Spec.AppId] = msgHash
	h.hashMu.Unlock()

	// 5. 记录日志
	b, _ := json.Marshal(msg)
	logger.Info("Pushed message to node",
		"appId", node.Spec.AppId,
		"eventType", "ConfigUpdate",
		"dataSize", len(b))

	return nil
}

func (h *NodeEventHandler) computeMessageHash(msg *internal.Message) string {
	data, _ := json.Marshal(msg)
	return fmt.Sprintf("%x", sha256.Sum256(data))
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

func (n *NodeEventHandler) pushMessage(appId string, msg *internal.Message) {
	logger := klog.Background()

	// 计算消息哈希
	msgHash := n.computeMessageHash(msg)

	// 检查是否与上次推送相同
	n.hashMu.RLock()
	lastHash, exists := n.lastPushedHash[appId]
	n.hashMu.RUnlock()

	if exists && lastHash == msgHash {
		logger.V(4).Info("Message unchanged, skip push", "appId", appId)
		return
	}

	// 推送
	if err := n.wt.Send(appId, msg); err != nil {
		logger.Error(err, "Failed to push message", "appId", appId)
		return
	}

	// 更新哈希缓存
	n.hashMu.Lock()
	n.lastPushedHash[appId] = msgHash
	n.hashMu.Unlock()

	b, _ := json.Marshal(msg)
	logger.Info("Pushed message", "appId", appId, "eventType", msg.EventType, "size", len(b))
}

// handleNodeDelete 处理 Node 删除
func (n *NodeEventHandler) handleNodeDelete(ctx context.Context, node *wireflowv1alpha1.Node) {
	logger := klog.FromContext(ctx)
	logger.Info("Node deleted", "node", node.Name)

	// 通知网络中的其他节点
	if len(node.Spec.Network) > 0 {
		for _, networkName := range node.Spec.Network {
			network, err := n.networkLister.Networks(node.Namespace).Get(networkName)
			if err != nil {
				logger.Error(err, "Failed to get network", "network", networkName)
				continue
			}

			n.notifyPeersNodeRemoved(network, node)
		}
	}

	// 清理哈希缓存
	n.hashMu.Lock()
	delete(n.lastPushedHash, node.Spec.AppId)
	n.hashMu.Unlock()
}

func (n *NodeEventHandler) notifyPeersNodeRemoved(network *wireflowv1alpha1.Network, node *wireflowv1alpha1.Node) {
	logger := klog.Background()
	logger.Info("Notify peers node removed", "network", network.Name, "node", node.Name)

	msg := &internal.Message{
		EventType: internal.EventTypeNodeRemove,
		Current: &internal.Node{
			Name:       node.Name,
			AppID:      node.Spec.AppId,
			Address:    node.Spec.Address,
			PublicKey:  node.Spec.PublicKey,
			PrivateKey: node.Spec.PrivateKey,
		},
	}

	objs, err := n.informer.Informer().GetIndexer().ByIndex("network", network.Name)
	if err != nil {
		logger.Error(err, "Failed to get network nodes", "network", network.Name)
		return
	}

	for _, obj := range objs {
		nodeObj := obj.(*wireflowv1alpha1.Node)
		if nodeObj.Name == nodeObj.Name {
			continue
		}

		n.pushMessage(nodeObj.Spec.AppId, msg)
	}
}

func (n *NodeEventHandler) IsConnected(ctx context.Context, node *wireflowv1alpha1.Node) bool {
	return node.Status.Status == wireflowv1alpha1.Active
}

func (n *NodeEventHandler) SpecEquals(old, new *wireflowv1alpha1.Node) bool {
	if old.Spec.PrivateKey != new.Spec.PrivateKey {
		return false
	}

	return true
}

func (n *NodeEventHandler) StatusEquals(old, new *wireflowv1alpha1.Node) bool {
	if old.Status.Status != new.Status.Status {
		return false
	}

	return true
}

// getNodeContext 获取节点的完整上下文
func (n *NodeEventHandler) getNodeContext(node *wireflowv1alpha1.Node) *NodeContext {
	if node == nil {
		return &NodeContext{}
	}

	ctx := &NodeContext{
		Node: node,
	}

	// 获取网络信息
	if len(node.Spec.Network) > 0 {
		networkName := node.Spec.Network[0]
		network, err := n.networkLister.Networks(node.Namespace).Get(networkName)
		if err == nil {
			ctx.Network = network

			// 获取 peers
			for _, nodeName := range network.Spec.Nodes {
				if nodeName == node.Name {
					continue
				}
				peer, err := n.nodeLister.Nodes(node.Namespace).Get(nodeName)
				if err == nil {
					ctx.Nodes = append(ctx.Nodes, peer)
				}
			}

			// 获取策略
			//policies, err := n.clientSet.WireflowcontrollerV1alpha1().
			//	NetworkPolicies(node.Namespace).
			//	List(context.Background(), metav1.ListOptions{
			//		LabelSelector: fmt.Sprintf("wireflow.io/network=%s", networkName),
			//	})

			policies, err := n.policyLister.NetworkPolicies(node.Namespace).List(labels.Everything())
			if err == nil {
				ctx.Policies = append(ctx.Policies, policies...)
			}
		}
	}

	return ctx
}

// buildFullConfigurationWithChanges 构建带变更详情的完整配置
func (n *NodeEventHandler) buildFullConfigurationWithChanges(
	node *wireflowv1alpha1.Node,
	context *NodeContext,
	changes *internal.ChangeDetails,
) (*internal.Message, error) {

	// 生成配置版本号
	version := n.generateConfigVersion()

	msg := &internal.Message{
		EventType:     internal.EventTypeNodeUpdate, // 统一使用 ConfigUpdate
		ConfigVersion: version,
		Timestamp:     time.Now().Unix(),
		Changes:       changes, // ← 携带变更详情
		Current: &internal.Node{
			Name:       node.Name,
			AppID:      node.Spec.AppId,
			Address:    node.Spec.Address,
			PublicKey:  node.Spec.PublicKey,
			PrivateKey: node.Spec.PrivateKey,
			//AllowedIPs: node.Spec.AllowIedPS,
		},
		Network: &internal.Network{
			Nodes:    make([]*internal.Node, 0),
			Policies: make([]*internal.Policy, 0),
		},
	}

	// 填充网络信息
	if context.Network != nil {
		msg.Network.NetworkId = context.Network.Name
		msg.Network.NetworkName = context.Network.Spec.Name
		//msg.Network.Address = context.Network.Spec.Address
		//msg.Network.Port = context.Network.Spec.Port

		// 填充 peers
		for _, peer := range context.Nodes {
			if peer.Spec.Address == "" {
				continue
			}

			msg.Network.Nodes = append(msg.Network.Nodes, &internal.Node{
				Name:       peer.Name,
				AppID:      peer.Spec.AppId,
				Address:    peer.Spec.Address,
				PublicKey:  peer.Spec.PublicKey,
				AllowedIPs: fmt.Sprintf("%s/32", peer.Spec.Address),
				//Endpoint:   peer.Spec.Endpoint,
			})
		}

		// 填充策略
		for _, policy := range context.Policies {
			msg.Network.Policies = append(msg.Network.Policies, &internal.Policy{
				PolicyName: policy.Name,
				// 填充规则
			})
		}
	}

	return msg, nil
}

// generateConfigVersion 生成配置版本号
func (n *NodeEventHandler) generateConfigVersion() string {
	n.versionMu.Lock()
	defer n.versionMu.Unlock()

	n.versionCounter++
	return fmt.Sprintf("v%d", n.versionCounter)
}
