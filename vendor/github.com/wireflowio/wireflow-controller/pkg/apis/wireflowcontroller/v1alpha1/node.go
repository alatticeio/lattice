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
}

// NodeStatus is the status for a Node resource
type NodeStatus struct {
	// Node status
	Status Status `json:"status,omitempty"`

	// Connection summary
	ConnectionSummary ConnectionSummary `json:"connectionSummary,omitempty"`

	// Connections states
	Connections []NodeConnection `json:"connections, omitempty"`
}

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
