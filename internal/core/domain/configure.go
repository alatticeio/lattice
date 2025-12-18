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

package domain

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Configurer is the interface for configuring WireGuard interfaces.
type Configurer interface {
	// ConfigureWG configures the WireGuard interface.
	Configure() error

	AddPeer(peer *SetPeer) error

	RemovePeer(peer *SetPeer) error

	RemoveAllPeers()

	ConfigSet(conf *DeviceConfig) error

	GetAddress() string

	GetIfaceName() string

	GetPeersManager() PeerManager
}

type SetPeer struct {
	PrivateKey           string
	PublicKey            string
	PresharedKey         string
	Endpoint             string
	AllowedIPs           string
	PersistentKeepalived int
	Remove               bool
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
	printf(&sb, "public_key", p.PublicKey, keyf)
	printf(&sb, "preshared_key", p.PresharedKey, keyf)
	printf(&sb, "replace_allowed_ips", strconv.FormatBool(true), nil)
	printf(&sb, "persistent_keepalive_interval", strconv.Itoa(p.PersistentKeepalived), nil)
	printf(&sb, "allowed_ip", p.AllowedIPs, nil)
	printf(&sb, "endpoint", p.Endpoint, nil)
	if p.Remove {
		printf(&sb, "remove", strconv.FormatBool(p.Remove), nil)
	}

	return sb.String()
}
