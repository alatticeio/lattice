// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infra

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Config struct {
	DrpHttpListen string `json:"drp_http_listen"`
}

// DeviceConf will used to fetchPeers,then config to the device
type DeviceConf struct {
	DrpUrl string        `json:"drpUrl,omitempty"` // a drp server user created.
	Device *DeviceConfig `json:"device,omitempty"`
	Nodes  []*Peer       `json:"list,omitempty"`
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
	if d.Device != nil {
		//sb.WriteString("set=1\n")
		printf(&sb, "private_key", d.Device.PrivateKey, keyf)
		printf(&sb, "listen_port", strconv.Itoa(d.Device.ListenPort), nil)
		printf(&sb, "fwmark", strconv.Itoa(d.Device.Fwmark), nil)
	}

	if d.Nodes != nil {
		for _, peer := range d.Nodes {
			printf(&sb, "public_key", peer.PublicKey, keyf)
			printf(&sb, "preshared_key", peer.PresharedKey, keyf)
			printf(&sb, "replace_allowed_ips", strconv.FormatBool(true), nil)
			printf(&sb, "allowed_ip", peer.AllowedIPs, nil)
			printf(&sb, "endpoint", peer.Endpoint, nil)
			//sb.WriteString(fmt.Sprintf("public_key=%s\n", keyf(peer.RemoteKey)))
			//sb.WriteString(fmt.Sprintf("preshared_key=%s\n", keyf(peer.PresharedKey)))
			//sb.WriteString(fmt.Sprintf("replace_allow_ips=%t\n", peer.ReplacePeers))
			//sb.WriteString(fmt.Sprintf("allow_ips=%s\n", peer.AllowedIps))
			//sb.WriteString(fmt.Sprintf("endpoint=%s\n", peer.Endpoint))
		}
	}

	return sb.String()
}

// Parse will generate a DeviceConf from a "wg8" get format
func (d *DeviceConf) Parse(str string) (*DeviceConf, error) {

	var err error
	deviceConfig := true

	conf := &DeviceConf{
		Device: &DeviceConfig{},
		Nodes:  make([]*Peer, 0),
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
			conf.Nodes = append(conf.Nodes, setPeer.peer)
			continue
		}

		if deviceConfig {
			switch key {
			case "private_key":
				conf.Device.PrivateKey = value
			case "listen_port":
				conf.Device.ListenPort, err = strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid ListenPort!!!, err:%v", line)
				}
			case "fwmark":
				conf.Device.Fwmark, err = strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid Fwmark!!!, err:%v", line)
				}
			case "replace_peers":
				conf.Device.ReplacePeers, err = strconv.ParseBool(value)
				if err != nil {
					return nil, fmt.Errorf("invalid replace_peers!!!, err:%v", line)
				}
			}
		} else {
			switch key {
			case "preshared_key":
				setPeer.peer.PresharedKey = value
			case "replace_allow_ips":
				setPeer.peer.ReplacePeers, err = strconv.ParseBool(value)
				if err != nil {
					return nil, fmt.Errorf("invalid replace_peers!!!, err:%v", line)
				}
			case "allow_ips":
				setPeer.peer.AllowedIPs = value
			}
		}
	}

	return conf, nil
}

type setPeer struct {
	peer *Peer
}
