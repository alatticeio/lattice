package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Node is a specification for a wireflow Node resource
type Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSpec   `json:"spec"`
	Status NodeStatus `json:"status"`
}

// NodeSpec is the spec for a Node resource
type NodeSpec struct {
	AppId string `json:"appId"`

	// node belongs to which user
	Username string `json:"username"`

	// node belongs to which policy
	Policy string `json:"policy"`

	// node's public key
	PrivateKey string `json:"privateKey"`

	PublicKey string `json:"publicKey"`

	Tags []string `json:"tags"`

	allowedIPs []string `json:"allowedIPs"`

	// node name for every node
	NodeName string `json:"nodeName"`

	// a network the node has joined in, some time a node may have multiple networks
	Network []string `json:"network"`

	// node ip, when a node is created, it will have a ip, and it will change when the network is changed
	Address string `json:"address"`

	//labels for node
	Labels []string `json:"labels"`
}

// NodeStatus is the status for a Node resource
type NodeStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Node status
	Status Status `json:"status,omitempty"`

	Phase NodePhase `json:"phase,omitempty"`

	ActiveNetworks []string `json:"activeNetworks,omitempty"`

	AllocatedAddress string `json:"allocatedAddress,omitempty"`

	// Connection summary
	ConnectionSummary ConnectionSummary `json:"connectionSummary,omitempty"`

	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObserveGeneration is the generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type NodePhase string

const (
	// NodePending 节点刚创建,等待处理
	NodePending NodePhase = "Pending"

	// NodeProvisioning 正在为节点分配资源(IP等)
	NodeProvisioning NodePhase = "Provisioning"

	// NodeReady 节点已就绪,网络配置完成
	NodeReady NodePhase = "Ready"

	// NodeUpdating 节点配置正在更新，比如节点配置策略，添加Label等
	NodeUpdating NodePhase = "Updating"

	NodeUpdatingPolicy NodePhase = "UpdatingPolicy"

	// NodeTerminating 节点正在离开网络/清理资源
	NodeTerminating NodePhase = "Terminating"

	// NodeFailed 节点处于错误状态
	NodeFailed NodePhase = "Failed"

	//部分功能不可用
	NodeDegraded NodePhase = "Degraded"
)

// Condition Types
const (
	NodeConditionInitialized = "Initialized"

	// NodeConditionProvisioned 节点是否就绪
	NodeConditionProvisioned = "Provisioned"

	// NodeConditionNetworkConfigured 网络配置是否完成
	NodeConditionNetworkConfigured = "NetworkConfigured"

	// NodeConditionIPAllocated IP 是否已分配
	NodeConditionIPAllocated = "IPAllocated"

	NodeConditionPolicyUpdating = "PolicyUpdating"

	// NodeConditionPolicyApplied 策略是否已应用
	NodeConditionPolicyApplied = "PolicyApplied"
)

// Condition Reasons
const (
	ReasonInitializing     = "Initializing"
	ReasonAllocating       = "Allocating"
	ReasonConfiguring      = "Configuring"
	ReasonReady            = "Ready"
	ReasonNotReady         = "NotReady"
	ReasonUpdating         = "Updating"
	ReasonLeaving          = "Leaving"
	ReasonAllocationFailed = "AllocationFailed"
	ReasonConfigFailed     = "ConfigurationFailed"
)

type Status string

const (
	Active   Status = "Active"
	InActive Status = "inactive"
	Stopped  Status = "stopped"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeList is a list of Node resources
type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Node `json:"items"`
}

// ConnectionSummary represents connection summary
type ConnectionSummary struct {
	Total        int `json:"total"`
	Connected    int `json:"connected"`
	Disconnected int `json:"disconnected"`
}
