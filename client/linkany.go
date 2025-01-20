package client

import (
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"linkany/pkg/config"
	"time"
)

func PeerToPeerConfig(peers []config.Peer) []wgtypes.PeerConfig {
	var wgPeers []wgtypes.PeerConfig
	for _, peer := range peers {
		pubKey, err := ParseKey(peer.PublicKey)
		if err != nil {
			continue
		}
		t := time.Duration(25) * time.Second
		wgPeer := wgtypes.PeerConfig{
			PublicKey:                   pubKey,
			Endpoint:                    nil,
			PersistentKeepaliveInterval: &t,
		}

		wgPeers = append(wgPeers, wgPeer)
	}

	return wgPeers
}

func ParseKey(key string) (wgtypes.Key, error) {
	return wgtypes.ParseKey(key)
}
