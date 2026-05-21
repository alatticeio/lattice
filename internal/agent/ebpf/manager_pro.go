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

//go:build pro && linux

package ebpf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// Manager loads and manages the eBPF TC ingress program for policy enforcement.
type Manager struct {
	logger  *log.Logger
	iface   string
	objs    *tc_ingressObjects
	tcxLink link.Link
	mu      sync.Mutex
}

// NewManager creates a new eBPF Manager for the given interface.
func NewManager(iface string, logger *log.Logger) *Manager {
	return &Manager{
		iface:  iface,
		logger: logger,
	}
}

// Load loads the eBPF programs and attaches the TC ingress classifier.
func (m *Manager) Load() error {
	objs := &tc_ingressObjects{}
	if err := loadTc_ingressObjects(objs, nil); err != nil {
		return fmt.Errorf("loading tc_ingress objects: %w", err)
	}
	m.objs = objs

	// Default action: DROP (0).
	var key uint32
	action := uint8(ActionDrop)
	if err := objs.DefaultActionMap.Put(&key, &action); err != nil {
		objs.Close()
		return fmt.Errorf("setting default action: %w", err)
	}

	iface, err := net.InterfaceByName(m.iface)
	if err != nil {
		objs.Close()
		return fmt.Errorf("getting interface %s: %w", m.iface, err)
	}

	lk, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   objs.LatticeTcIngress,
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		objs.Close()
		return fmt.Errorf("attaching TCX ingress to %s: %w", m.iface, err)
	}
	m.tcxLink = lk

	m.logger.Info("ebpf manager loaded", "iface", m.iface)
	return nil
}

// Provision applies firewall rules to the BPF maps.
func (m *Manager) Provision(rule *infra.FirewallRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.objs == nil {
		return errors.New("ebpf manager not loaded")
	}

	for _, r := range rule.Ingress {
		if err := m.applyRules(r, m.objs.IngressPolicyMap, m.objs.IngressPortPolicyMap); err != nil {
			return fmt.Errorf("provision ingress: %w", err)
		}
	}

	for _, r := range rule.Egress {
		if err := m.applyRules(r, m.objs.IngressPolicyMap, m.objs.IngressPortPolicyMap); err != nil {
			return fmt.Errorf("provision egress: %w", err)
		}
	}

	return nil
}

func (m *Manager) applyRules(r infra.TrafficRule, ipMap, portMap *ebpf.Map) error {
	action := uint8(ActionDrop)
	if r.Action == "Accept" {
		action = ActionAccept
	}

	for _, peer := range r.Peers {
		ip := net.ParseIP(peer)
		if ip == nil {
			continue
		}
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}

		srcIP := binary.BigEndian.Uint32(ip4)

		if r.Protocol != "" && r.Port > 0 {
			pkey := tc_ingressPortKey{
				LpmKey:   32,
				SrcIp:    srcIP,
				Protocol: protoToNumber(r.Protocol),
				DstPort:  uint16(r.Port),
			}
			if err := portMap.Put(&pkey, &action); err != nil {
				return fmt.Errorf("port policy put: %w", err)
			}
		} else {
			ikey := tc_ingressIpKey{
				LpmKey: 32,
				SrcIp:  srcIP,
			}
			if err := ipMap.Put(&ikey, &action); err != nil {
				return fmt.Errorf("ip policy put: %w", err)
			}
		}
	}
	return nil
}

func protoToNumber(p string) uint8 {
	switch p {
	case "tcp":
		return 6
	case "udp":
		return 17
	default:
		return 0
	}
}

// Cleanup detaches the TC program and closes all eBPF resources.
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs error
	if m.tcxLink != nil {
		if err := m.tcxLink.Close(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("tcx link close: %w", err))
		}
		m.tcxLink = nil
	}
	if m.objs != nil {
		m.objs.Close()
		m.objs = nil
	}
	return errs
}

// Name returns "ebpf".
func (m *Manager) Name() string { return "ebpf" }

// SetupNAT is a no-op for eBPF; NAT is handled by iptables.
func (m *Manager) SetupNAT(_ string) error { return nil }
