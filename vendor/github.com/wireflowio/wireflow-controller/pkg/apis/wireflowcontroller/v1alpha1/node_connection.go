package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeConnection represents a connection view between two nodes
type NodeConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeConnectionSpec   `json:"spec"`
	Status NodeConnectionStatus `json:"status"`
}

// NodeConnectionSpec defines the desired state of NodeConnection
type NodeConnectionSpec struct {
	// Local node reference
	NodeRef NodeRef `json:"nodeRef"`

	// Peer node reference
	PeerRef NodeRef `json:"peerRef"`

	// Connection configuration
	Config *ConnectionConfig `json:"config,omitempty"`
}

// NodeRef references a node
type NodeRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ConnectionConfig defines connection configuration
type ConnectionConfig struct {
	Protocol          string `json:"protocol,omitempty"`
	Port              int32  `json:"port,omitempty"`
	AutoReconnect     bool   `json:"autoReconnect"`
	ReconnectInterval int32  `json:"reconnectInterval,omitempty"`
}

// NodeConnectionStatus defines the observed state of NodeConnection
type NodeConnectionStatus struct {
	// Connection state (synced from Node status)
	State ConnectionState `json:"state,omitempty"`

	// Peer endpoint
	Endpoint string `json:"endpoint,omitempty"`

	// Connection latency
	Latency string `json:"latency,omitempty"`

	// Last handshake time
	LastHandshake *metav1.Time `json:"lastHandshake,omitempty"`

	// Traffic statistics
	Traffic *ConnectionTraffic `json:"traffic,omitempty"`

	// Last sync time from Node status
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Human-readable message
	Message string `json:"message,omitempty"`
}

// ConnectionState represents connection state
type ConnectionState string

const (
	ConnectionStateConnected    ConnectionState = "Connected"
	ConnectionStateDisconnected ConnectionState = "Disconnected"
	ConnectionStateConnecting   ConnectionState = "Connecting"
	ConnectionStateUnknown      ConnectionState = "Unknown"
)

// ConnectionTraffic defines traffic statistics
type ConnectionTraffic struct {
	BytesReceived int64 `json:"bytesReceived,omitempty"`
	BytesSent     int64 `json:"bytesSent,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeConnectionList is a list of NodeConnection resources
type NodeConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NodeConnection `json:"items"`
}
