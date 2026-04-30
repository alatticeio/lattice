package vo

// WorkspaceEndpointVo describes one side of a peering connection.
type WorkspaceEndpointVo struct {
	Name      string `json:"name"`      // workspace display name
	Namespace string `json:"namespace"` // K8s namespace
	CIDR      string `json:"cidr"`      // active network CIDR
	NodeCount int    `json:"nodeCount"` // real (non-shadow) peer count
}

// PeeringVo is the HTTP response type for a LatticeNetworkPeering resource.
type PeeringVo struct {
	Name        string              `json:"name"`
	Local       WorkspaceEndpointVo `json:"local"`
	Remote      WorkspaceEndpointVo `json:"remote"`
	Status      string              `json:"status"`      // active | pending | failed
	PeeringMode string              `json:"peeringMode"` // gateway | mesh
	CreatedAt   string              `json:"createdAt"`
}
