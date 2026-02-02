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
	"wireflow/internal/log"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	PlatformMacOS   = "darwin"
)

// Provisioner is the interface for configuring WireGuard interfaces.
type Provisioner interface {
	RouteProvisioner
	RuleProvisioner
	// ConfigureWG configures the WireGuard interface.
	SetupInterface(conf *DeviceConfig) error

	AddPeer(peer *SetPeer) error

	RemovePeer(peer *SetPeer) error

	RemoveAllPeers()

	GetAddress() string

	GetIfaceName() string
}

type RouteProvisioner interface {
	ApplyRoute(action, address, name string) error
	ApplyIP(action, address, name string) error
}

type RuleProvisioner interface {
	// Name 返回执行器的名称（如 "iptables", "nftables", "windows-fw"）
	Name() string

	// Provision 接收结构化规则并应用到系统中
	Provision(rule *FirewallRule) error

	// Cleanup 清理所有由该执行器创建的规则
	Cleanup() error

	// for docker setup nat and other iptables rules
	SetupNAT(interfaceName string) error
}

const (
	PersistentKeepalive int = 25
)

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

type provisioner struct {
	RouteProvisioner
	RuleProvisioner
	device    *wg.Device
	address   string
	ifaceName string
}

func (p *provisioner) GetAddress() string {
	return p.address
}

func (p *provisioner) GetIfaceName() string {
	return p.ifaceName
}

type Params struct {
	Device    *wg.Device
	IfaceName string
	Address   string
}

func (p *provisioner) SetupInterface(conf *DeviceConfig) error {
	return p.device.IpcSet(conf.String())
}

func (p *provisioner) AddPeer(peer *SetPeer) error {
	return p.device.IpcSet(peer.String())
}

func (p *provisioner) RemovePeer(peer *SetPeer) error {
	return p.device.IpcSet(peer.String())
}

func (p *provisioner) RemoveAllPeers() {
	p.device.RemoveAllPeers()
}

func NewProvisioner(routeProvisioner RouteProvisioner, ruleProvisioner RuleProvisioner, config *Params) Provisioner {
	return &provisioner{
		RouteProvisioner: routeProvisioner,
		RuleProvisioner:  ruleProvisioner,
		device:           config.Device,
		address:          config.Address,
		ifaceName:        config.IfaceName,
	}
}

type routeProvisioner struct {
	logger *log.Logger
}

func NewRouteProvisioner(logger *log.Logger) RouteProvisioner {
	return &routeProvisioner{
		logger: logger,
	}
}

type ruleProvisioner struct {
	interfaceName string
	logger        *log.Logger
}

func NewRuleProvisioner(logger *log.Logger) RuleProvisioner {
	return &ruleProvisioner{
		logger: logger,
	}
}
