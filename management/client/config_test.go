package client

import (
	"fmt"
	"linkany/pkg/config"
	"testing"
)

func TestDeviceConf_String(t *testing.T) {
	d := &config.DeviceConf{
		Device: &config.DeviceConfig{
			PrivateKey:   "vVqSz6YQor7p//Shkgu7aHj6HoosXyrx9UlPhbwoDzs=",
			ListenPort:   51820,
			Fwmark:       0,
			ReplacePeers: true,
		},
		Peers: []*config.Peer{
			{
				PublicKey:    "vVqSz6YQor7p//Shkgu7aHj6HoosXyrx9UlPhbwoDzs=",
				PresharedKey: "vVqSz6YQor7p//Shkgu7aHj6HoosXyrx9UlPhbwoDzs=",
				Endpoint:     "10.0.0.1:51820",
				ReplacePeers: true,
				AllowedIps:   "10.0.0.2/32",
			},
		},
	}

	fmt.Println(d.String())
}

func TestDeviceConf_Parse(t *testing.T) {

}
