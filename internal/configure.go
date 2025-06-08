package internal

import (
	"linkany/pkg/config"
)

// ConfigureManager is the interface for configuring WireGuard interfaces.
type ConfigureManager interface {
	// ConfigureWG configures the WireGuard interface.
	ConfigureWG() error

	AddPeer(peer *SetPeer) error

	GetAddress() string

	GetIfaceName() string

	GetPeersManager() *config.NodeManager

	RemovePeer(peer *SetPeer) error
	//
	//AddAllowedIPs(peer *SetPeer) error
	//
	//RemoveAllowedIPs(peer *SetPeer) error
}
