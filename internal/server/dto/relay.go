package dto

// RelayDto carries relay server fields from HTTP request body.
type RelayDto struct {
	// Name is the unique CRD resource name (slug-style, e.g. "asia-hk-01").
	Name string `json:"name" binding:"required,min=1,max=64"`

	DisplayName string `json:"displayName" binding:"required,min=1,max=128"`
	Description string `json:"description,omitempty"`
	TcpUrl      string `json:"tcpUrl" binding:"required,url"`
	QuicUrl     string `json:"quicUrl,omitempty"`
	Enabled     bool   `json:"enabled"`

	// Workspaces holds workspace IDs (from the DB) to bind this relay.
	// Empty means all workspaces.
	Workspaces []string `json:"workspaces,omitempty"`
}
