package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network is using for wireflow network, a node join a network, and a network has many nodes
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status"`
}

// NetworkSpec is the spec for a Node resource
type NetworkSpec struct {
	// name of network
	Name string `json:"name,omitempty"`

	NetworkId string `json:"networkId,omitempty"`

	Owner string `json:"owner,omitempty"`

	CIDR string `json:"cidr,omitempty"`

	Mtu int `json:"mtu,omitempty"`

	Dns string `json:"dns,omitempty"`

	Nodes []string `json:"nodes,omitempty"`

	// 已分配的 IP 列表
	AllocatedIPs []IPAllocation `json:"allocatedIPs,omitempty"`

	// 可用 IP 数量
	AvailableIPs int `json:"availableIPs,omitempty"`
}

type Dns struct {
	Enabled bool     `json:"enabled"`
	Servers []string `json:"servers"`
}

// NodeStatus is the status for a Node resource
type NetworkStatus struct {
	// Node status
	Status Status `json:"status,omitempty"`

	// Connection summary
	ConnectionSummary ConnectionSummary `json:"connectionSummary,omitempty"`

	// Connections states
	Connections []NodeConnection `json:"connections, omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeList is a list of Node resources
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Network `json:"items"`
}

// IPAllocation IP 分配记录
type IPAllocation struct {
	IP          string      `json:"ip"`
	Node        string      `json:"node"`
	AllocatedAt metav1.Time `json:"allocatedAt"`
}

type DNSConfig struct {
	Enabled bool     `json:"enabled"`
	Servers []string `json:"servers,omitempty"`
}
