package iface

import "linkany/pkg/config"

// WGConfigureInterface is the interface for configuring WireGuard interfaces.
type WGConfigureInterface interface {
	// ConfigureWG configures the WireGuard interface.
	ConfigureWG() error

	AddPeer(peer *SetPeer) error

	GetAddress() string

	GetIfaceName() string

	GetPeersManager() *config.PeersManager

	//RemovePeer(peer *SetPeer) error
	//
	//AddAllowedIPs(peer *SetPeer) error
	//
	//RemoveAllowedIPs(peer *SetPeer) error
}
