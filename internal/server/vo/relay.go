package vo

import "time"

// RelayTestVo is returned by the connectivity probe endpoint.
type RelayTestVo struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

// RelayVo is the read model returned to the frontend.
type RelayVo struct {
	// ID is the CRD resource name.
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	TcpUrl      string `json:"tcpUrl"`
	QuicUrl     string `json:"quicUrl,omitempty"`
	Enabled     bool   `json:"enabled"`

	// Status mirrors LatticeRelayServerStatus.Health in lower-case.
	Status string `json:"status,omitempty"`

	// LatencyMs is the last probe round-trip latency.
	LatencyMs *int64 `json:"latencyMs,omitempty"`

	// ConnectedPeers is the number of peers configured to use this relay.
	ConnectedPeers int `json:"connectedPeers,omitempty"`

	// Workspaces holds the K8s namespace names the relay is scoped to.
	// Empty slice means all workspaces.
	Workspaces []string `json:"workspaces,omitempty"`

	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}
