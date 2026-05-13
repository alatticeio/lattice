//go:build pro

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

package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alatticeio/lattice-shim/shim"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type sandboxCloser struct {
	sb       *gvisor.Sandbox
	auditF   *os.File
	wgBind   shim.WireGuardBind
	wgDevice *device.Device
}

func (c *sandboxCloser) Close() {
	if c.wgDevice != nil {
		c.wgDevice.Close()
	}
	if c.sb != nil {
		c.sb.Close()
	}
	if c.wgBind != nil {
		c.wgBind.Close()
	}
	if c.auditF != nil {
		c.auditF.Close()
	}
}

// createSandbox creates a gVisor sandbox with optional wireguard-go attachment.
// sandboxName and localIP are required. agentJWT may be empty.
// When wgEnabled is true, a UDP bind on :51820 and a wireguard-go device are
// created. The privateKey is the sandbox's WireGuard private key, and peers
// are the initial set of static peers to configure.
func createSandbox(sandboxName, localIP, agentJWT string, wgEnabled bool, privateKey wgtypes.Key, peers []wgtypes.PeerConfig) (*sandboxCloser, error) {
	var policyChecker shim.PolicyChecker
	var auditWriter shim.AuditWriter
	var auditFile *os.File

	// Open audit JSONL file so e2e tests can read it.
	auditPath := "/tmp/lattice-audit.jsonl"
	f, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[sandbox] cannot open audit file %s: %v (audit disabled)", auditPath, err)
	} else {
		auditFile = f
		fileWriter := &fileAuditWriter{f: f}
		auditWriter = gvisor.NewAuditAdapter(sandboxName, fileWriter)
	}

	cfg := gvisor.Config{
		ID:            sandboxName,
		LocalIP:       localIP,
		PolicyChecker: policyChecker,
		AuditWriter:   auditWriter,
	}

	var wgBind shim.WireGuardBind
	var wgDev *device.Device

	var wgBindAdapter conn.Bind

	if wgEnabled {
		wgBind, wgBindAdapter, err = gvisor.NewUDPBindWithBind(":51820")
		if err != nil {
			if auditFile != nil {
				auditFile.Close()
			}
			return nil, fmt.Errorf("create WG bind: %w", err)
		}

		// Do NOT set cfg.WireGuardBind — with wireguard-go we bypass the
		// pumpOutbound path and use the tun adapter + conn.Bind instead.
	}

	sb, err := gvisor.New(cfg)
	if err != nil {
		if wgBind != nil {
			wgBind.Close()
		}
		if auditFile != nil {
			auditFile.Close()
		}
		return nil, err
	}

	if wgEnabled {
		// Create tun adapter from gVisor channel endpoint.
		tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

		// Create wireguard-go logger.
		wgLogger := &device.Logger{
			Verbosef: func(format string, args ...any) {
				log.Printf("[wg] "+format, args...)
			},
			Errorf: func(format string, args ...any) {
				log.Printf("[wg ERROR] "+format, args...)
			},
		}

		// Create wireguard-go device.
		wgDev = device.NewDevice(tunDev, wgBindAdapter, wgLogger)

		// Configure the WireGuard device via UAPI.
		conf := formatWGConfig(privateKey, peers)
		if err := wgDev.IpcSet(conf); err != nil {
			wgDev.Close()
			sb.Close()
			wgBind.Close()
			if auditFile != nil {
				auditFile.Close()
			}
			return nil, fmt.Errorf("configure wireguard: %w", err)
		}

		log.Printf("[sandbox] wireguard-go device created for %q", sandboxName)
	}

	return &sandboxCloser{sb: sb, auditF: auditFile, wgBind: wgBind, wgDevice: wgDev}, nil
}

// formatWGConfig builds a WireGuard UAPI configuration string.
//
// The UAPI format uses key=value pairs, one per line. Device-level keys come
// first, then each peer is introduced with "public_key=" followed by its
// properties. An empty line terminates the configuration.
//
// See https://www.wireguard.com/xplatform/#configuration-protocol
func formatWGConfig(privateKey wgtypes.Key, peers []wgtypes.PeerConfig) string {
	var sb strings.Builder

	// Device-level: private key (hex-encoded for UAPI).
	fmt.Fprintf(&sb, "private_key=%s\n", hex.EncodeToString(privateKey[:]))

	for _, p := range peers {
		fmt.Fprintf(&sb, "public_key=%s\n", hex.EncodeToString(p.PublicKey[:]))
		if p.Endpoint != nil {
			fmt.Fprintf(&sb, "endpoint=%s\n", p.Endpoint.String())
		}
		for _, ip := range p.AllowedIPs {
			fmt.Fprintf(&sb, "allowed_ip=%s\n", ip.String())
		}
		if p.PersistentKeepaliveInterval != nil {
			secs := int(p.PersistentKeepaliveInterval.Seconds())
			fmt.Fprintf(&sb, "persistent_keepalive_interval=%d\n", secs)
		}
	}

	// Blank line terminates the UAPI set operation.
	sb.WriteByte('\n')
	return sb.String()
}

// fileAuditWriter writes audit events as JSONL to an open file.
type fileAuditWriter struct {
	f *os.File
}

func (w *fileAuditWriter) WriteAudit(agentID string, event shim.AuditEvent) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n, err := w.f.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return fmt.Errorf("short write: %d < %d", n, len(b))
	}
	return nil
}

// compile-time checks
var _ gvisor.AuditEventWriter = (*fileAuditWriter)(nil)
