package iface

import (
	"encoding/hex"
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"linkany/pkg/config"
	"strconv"
	"strings"
)

type SetPeer struct {
	PrivateKey           wgtypes.Key
	PublicKey            wgtypes.Key
	PresharedKey         wgtypes.Key
	Endpoint             string
	AllowedIPs           string
	PersistentKeepalived int
}

func (p *SetPeer) String() string {
	keyf := func(value string) string {
		if value == "" {
			return ""
		}
		result, err := wgtypes.ParseKey(value)
		if err != nil {
			return ""
		}

		return hex.EncodeToString(result[:])
	}

	printf := func(sb *strings.Builder, key, value string, keyf func(string) string) {

		if keyf != nil {
			value = keyf(value)
		}

		if value != "" {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	var sb strings.Builder
	printf(&sb, "public_key", p.PublicKey.String(), keyf)
	printf(&sb, "preshared_key", p.PresharedKey.String(), keyf)
	printf(&sb, "replace_allowed_ips", strconv.FormatBool(true), nil)
	printf(&sb, "persistent_keepalive_interval", strconv.Itoa(p.PersistentKeepalived), nil)
	printf(&sb, "allowed_ip", p.AllowedIPs, nil)
	printf(&sb, "endpoint", p.Endpoint, nil)

	return sb.String()
}

var (
	_ WGConfigure = (*WGConfiger)(nil)
)

type WGConfiger struct {
	device       *wg.Device
	address      string
	ifaceName    string
	peersManager *config.PeersManager
}

func (w *WGConfiger) GetAddress() string {
	return w.address
}

func (w *WGConfiger) GetIfaceName() string {
	return w.ifaceName
}

func (w *WGConfiger) GetPeersManager() *config.PeersManager {
	return w.peersManager
}

type WGConfigerParams struct {
	Device       *wg.Device
	IfaceName    string
	Address      string
	PeersManager *config.PeersManager
}

func (w *WGConfiger) ConfigureWG() error {
	return nil
}

func (w *WGConfiger) AddPeer(peer *SetPeer) error {
	return w.device.IpcSet(peer.String())
}

func NewWgConfiger(config *WGConfigerParams) *WGConfiger {
	return &WGConfiger{
		device:       config.Device,
		address:      config.Address,
		ifaceName:    config.IfaceName,
		peersManager: config.PeersManager,
	}
}
