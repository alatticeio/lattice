//go:build pro

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gvisor

import (
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/provision"
	wgdevice "golang.zx2c4.com/wireguard/device"
)

// sandboxProvisioner implements provision.Provisioner for the gVisor sandbox.
//
// WireGuard peer operations (SetupInterface, AddPeer, RemovePeer) are
// delegated to wireguard-go's IpcSet so the in-process wireguard-go device
// receives the configuration. Route, IP, and policy operations are no-ops
// because gVisor manages its own routing table internally and the EgressFilter
// passed to shim.NewNetstack handles policy enforcement.
type sandboxProvisioner struct {
	device    *wgdevice.Device
	localIP   string
	ifaceName string
}

// NewSandboxProvisionerFactory returns a ProvisionerFactory for NodeConfig.
// The factory is called by NewNode after the wireguard-go device is created,
// giving the provisioner a reference to the live device.
func NewSandboxProvisionerFactory(localIP, ifaceName string) func(*wgdevice.Device) provision.Provisioner {
	return func(dev *wgdevice.Device) provision.Provisioner {
		return &sandboxProvisioner{
			device:    dev,
			localIP:   localIP,
			ifaceName: ifaceName,
		}
	}
}

// SetupInterface configures the wireguard-go device (private key, listen port).
func (p *sandboxProvisioner) SetupInterface(conf *infra.DeviceConfig) error {
	return p.device.IpcSet(conf.String())
}

// AddPeer adds a WireGuard peer to the wireguard-go device.
func (p *sandboxProvisioner) AddPeer(peer *provision.SetPeer) error {
	return p.device.IpcSet(peer.String())
}

// RemovePeer removes a WireGuard peer from the wireguard-go device.
func (p *sandboxProvisioner) RemovePeer(peer *provision.SetPeer) error {
	return p.device.IpcSet(peer.String())
}

// RemoveAllPeers removes all WireGuard peers from the wireguard-go device.
func (p *sandboxProvisioner) RemoveAllPeers() {
	p.device.RemoveAllPeers()
}

// GetAddress returns the sandbox's VPN IP address.
func (p *sandboxProvisioner) GetAddress() string { return p.localIP }

// GetIfaceName returns a logical interface name used for logging.
func (p *sandboxProvisioner) GetIfaceName() string { return p.ifaceName }

// ApplyRoute is a no-op: gVisor manages its own routing table.
func (p *sandboxProvisioner) ApplyRoute(_, _, _ string) error { return nil }

// ApplyIP is a no-op: the gVisor netstack is configured with the IP at creation time.
func (p *sandboxProvisioner) ApplyIP(_, _, _ string) error { return nil }

// Name returns the enforcer name.
func (p *sandboxProvisioner) Name() string { return "gvisor" }

// Provision is a no-op: egress policy is handled by shim.EgressFilter injected
// into the gVisor netstack at sandbox creation time.
func (p *sandboxProvisioner) Provision(_ *infra.FirewallRule) error { return nil }

// Cleanup is a no-op: the gVisor netstack is torn down via Sandbox.Close().
func (p *sandboxProvisioner) Cleanup() error { return nil }

// SetupNAT is a no-op: gVisor does not use kernel NAT rules.
func (p *sandboxProvisioner) SetupNAT(_ string) error { return nil }

// Compile-time check.
var _ provision.Provisioner = (*sandboxProvisioner)(nil)
