// Copyright 2026 The Lattice Authors, Inc.
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

package wireguard

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const handshakeActiveThreshold = 3 * time.Minute

// PrintStatus prints the current WireGuard interface and peer status to stdout.
func PrintStatus(interfaceName string) error {
	ctr, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl: %w", err)
	}
	defer ctr.Close()

	var devices []*wgtypes.Device
	if interfaceName != "" {
		dev, err := ctr.Device(interfaceName)
		if err != nil {
			return fmt.Errorf("interface %q not found: %w", interfaceName, err)
		}
		devices = []*wgtypes.Device{dev}
	} else {
		devices, err = ctr.Devices()
		if err != nil {
			return fmt.Errorf("failed to list WireGuard devices: %w", err)
		}
		if len(devices) == 0 {
			return fmt.Errorf("lattice is not running (no WireGuard interfaces found)")
		}
	}

	for _, dev := range devices {
		iface, err := net.InterfaceByName(dev.Name)
		var addrs []string
		if err == nil {
			ifAddrs, _ := iface.Addrs()
			for _, a := range ifAddrs {
				addrs = append(addrs, a.String())
			}
		}

		fmt.Printf("Interface : %s\n", dev.Name)
		if len(addrs) > 0 {
			fmt.Printf("Address   : %s\n", strings.Join(addrs, ", "))
		}
		fmt.Printf("Public Key: %s\n", dev.PublicKey.String())
		fmt.Printf("Port      : %d\n", dev.ListenPort)

		connected := 0
		for _, p := range dev.Peers {
			if !p.LastHandshakeTime.IsZero() && time.Since(p.LastHandshakeTime) < handshakeActiveThreshold {
				connected++
			}
		}
		fmt.Printf("\nPeers: %d total, %d connected\n", len(dev.Peers), connected)

		for _, p := range dev.Peers {
			printPeer(p)
		}
	}
	return nil
}

func printPeer(p wgtypes.Peer) {
	status := "disconnected"
	handshakeStr := "never"
	if !p.LastHandshakeTime.IsZero() {
		elapsed := time.Since(p.LastHandshakeTime)
		handshakeStr = formatDuration(elapsed) + " ago"
		if elapsed < handshakeActiveThreshold {
			status = "connected"
		}
	}

	var allowedIPs []string
	for _, ip := range p.AllowedIPs {
		allowedIPs = append(allowedIPs, ip.String())
	}
	ipStr := strings.Join(allowedIPs, ", ")
	if ipStr == "" {
		ipStr = "(none)"
	}

	endpointStr := "(none)"
	if p.Endpoint != nil {
		endpointStr = p.Endpoint.String()
	}

	fmt.Printf("\n  Peer      : %s\n", p.PublicKey.String())
	fmt.Printf("  Address   : %s\n", ipStr)
	fmt.Printf("  Endpoint  : %s\n", endpointStr)
	fmt.Printf("  Handshake : %s\n", handshakeStr)
	fmt.Printf("  Traffic   : ↑ %s  ↓ %s\n", formatBytes(p.TransmitBytes), formatBytes(p.ReceiveBytes))
	fmt.Printf("  Status    : %s\n", status)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return fmt.Sprintf("%d hours", int(d.Hours()))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
