package dto

type PeeringDto struct {
	// Name is optional; auto-generated as "{nsA}-to-{nsB}" when empty.
	Name string `json:"name,omitempty"`

	// NamespaceB is the remote workspace's K8s namespace.
	NamespaceB string `json:"namespaceB"`

	// NetworkB is the LatticeNetwork name in NamespaceB.
	// Defaults to "lattice-default-net" when empty.
	NetworkB string `json:"networkB,omitempty"`

	// PeeringMode controls traffic forwarding: "gateway" (default) or "mesh".
	PeeringMode string `json:"peeringMode,omitempty"`
}
