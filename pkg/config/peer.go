package config

import (
	"encoding/hex"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Config struct {
	DrpHttpListen string `json:"drp_http_listen"`
}

// DeviceConf will used to fetchPeers,then config to the device
type DeviceConf struct {
	DrpUrl string       `json:"drpUrl,omitempty"` // a drp server user created.
	Device DeviceConfig `json:"device,omitempty"`
	Peers  []*Peer      `json:"list,omitempty"`
}

// Peer peers sync from linkany server will be transfered to
type Peer struct {
	Lock                sync.Mutex
	Connected           atomic.Bool
	P2PFlag             atomic.Bool
	ConnectionState     atomic.Bool `json:"checkingStatue,omitempty"`
	Name                string      `json:"name,omitempty"`
	PrivateKey          string      `json:"privateKey,omitempty"`
	PublicKey           string      `json:"publicKey,omitempty"`
	Address             string      `json:"address,omitempty"`
	Remove              bool        `json:"remove,omitempty"`
	Endpoint            string      `json:"endpoint,omitempty"`
	TieBreaker          uint32      `json:"tieBreaker,omitempty"`
	PersistentKeepalive int         `json:"persistentKeepalive,omitempty"`
	AllowedIps          string      `json:"allowedIps,omitempty"`
	PresharedKey        string      `json:"presharedKey,omitempty"`
	ReplacePeers        bool        `json:"replacePeers,omitempty"`
	Port                int         `json:"port,omitempty"`
	Ufrag               string      `json:"ufrag,omitempty"`
	Pwd                 string      `json:"pwd,omitempty"`
	HostIP              string      `json:"hostIP,omitempty"`
	SrflxIP             string      `json:"srflxIP,omitempty"`
	RelayIP             string      `json:"relayIP,omitempty"`
	Status              int         `json:"status,omitempty"` // 1: online 0: offline
}

// DeviceConfig config for this device
type DeviceConfig struct {
	PrivateKey   string `json:"privateKey,omitempty"`
	Address      string `json:"address,omitempty"`
	Fwmark       int    `json:"fwmark,omitempty"`
	ListenPort   int    `json:"listenPort,omitempty"`
	ReplacePeers bool   `json:"replacePeers,omitempty"`
}

func (d *DeviceConfig) String() string {
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
	//sb.WriteString("set=1\n")
	printf(&sb, "private_key", d.PrivateKey, keyf)
	if d.ListenPort != 0 {
		printf(&sb, "listen_port", strconv.Itoa(d.ListenPort), nil)
	}
	printf(&sb, "fwmark", strconv.Itoa(d.Fwmark), nil)
	return sb.String()
}

// String will generate "wg8" get format
func (d *DeviceConf) String() string {

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
	//sb.WriteString("set=1\n")
	printf(&sb, "private_key", d.Device.PrivateKey, keyf)
	printf(&sb, "listen_port", strconv.Itoa(d.Device.ListenPort), nil)
	printf(&sb, "fwmark", strconv.Itoa(d.Device.Fwmark), nil)
	//sb.WriteString(fmt.Sprintf("private_key=%s\n", keyf(d.Device.privateKey)))
	//sb.WriteString(fmt.Sprintf("listen_port=%d\n", 51820))
	//sb.WriteString(fmt.Sprintf("fwmark=%d\n", d.Device.Fwmark))
	//sb.WriteString(fmt.Sprintf("replace_peers=%t\n", d.Device.ReplacePeers))

	for _, peer := range d.Peers {
		printf(&sb, "public_key", peer.PublicKey, keyf)
		printf(&sb, "preshared_key", peer.PresharedKey, keyf)
		printf(&sb, "replace_allowed_ips", strconv.FormatBool(true), nil)
		printf(&sb, "allowed_ip", peer.AllowedIps, nil)
		printf(&sb, "endpoint", peer.Endpoint, nil)
		//sb.WriteString(fmt.Sprintf("public_key=%s\n", keyf(peer.RemoteKey)))
		//sb.WriteString(fmt.Sprintf("preshared_key=%s\n", keyf(peer.PresharedKey)))
		//sb.WriteString(fmt.Sprintf("replace_allow_ips=%t\n", peer.ReplacePeers))
		//sb.WriteString(fmt.Sprintf("allow_ips=%s\n", peer.AllowedIps))
		//sb.WriteString(fmt.Sprintf("endpoint=%s\n", peer.Endpoint))
	}

	return sb.String()
}

func (p *Peer) String() string {
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
	printf(&sb, "public_key", p.PublicKey, keyf)
	printf(&sb, "preshared_key", p.PresharedKey, keyf)
	printf(&sb, "replace_allowed_ips", strconv.FormatBool(true), nil)
	printf(&sb, "persistent_keepalive_interval", strconv.Itoa(p.PersistentKeepalive), nil)
	printf(&sb, "allowed_ip", p.AllowedIps, nil)
	printf(&sb, "endpoint", p.Endpoint, nil)

	return sb.String()
}

// Parse will generate a DeviceConf from a "wg8" get format
func (d *DeviceConf) Parse(str string) (*DeviceConf, error) {

	var err error
	deviceConfig := true

	conf := &DeviceConf{
		Device: DeviceConfig{},
		Peers:  make([]*Peer, 0),
	}
	setPeer := new(setPeer)
	result := strings.Split(str, "\n")
	for _, line := range result {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid config!!!, err:%v", line)
		}

		if key == "RemoteKey" {
			deviceConfig = false
			setPeer.peer = &Peer{
				PublicKey: value,
			}
			conf.Peers = append(conf.Peers, setPeer.peer)
			continue
		}

		if deviceConfig {
			switch key {
			case "private_key":
				conf.Device.PrivateKey = value
				break
			case "listen_port":
				conf.Device.ListenPort, err = strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid ListenPort!!!, err:%v", line)
				}
				break
			case "fwmark":
				conf.Device.Fwmark, err = strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid Fwmark!!!, err:%v", line)
				}
				break
			case "replace_peers":
				conf.Device.ReplacePeers, err = strconv.ParseBool(value)
				if err != nil {
					return nil, fmt.Errorf("invalid replace_peers!!!, err:%v", line)
				}
				break
			}
		} else {
			switch key {
			case "preshared_key":
				setPeer.peer.PresharedKey = value
				break
			case "replace_allow_ips":
				setPeer.peer.ReplacePeers, err = strconv.ParseBool(value)
				if err != nil {
					return nil, fmt.Errorf("invalid replace_peers!!!, err:%v", line)
				}
				break
			case "allow_ips":
				setPeer.peer.AllowedIps = value
				break
			}
		}
	}

	return conf, nil
}

type setPeer struct {
	peer *Peer
}
