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

package agent

import (
	"context"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
)

type Handler interface {
	HandleEvent(ctx context.Context, msg *infra.Message) error
	ApplyFullConfig(ctx context.Context, msg *infra.Message) error
}

// event handler for lattice to handle event from management
type MessageHandler struct {
	deviceManager infra.NodeInterface
	logger        *log.Logger
	provisioner   provision.Provisioner
}

func NewMessageHandler(e infra.NodeInterface, logger *log.Logger, provisioner provision.Provisioner) *MessageHandler {
	return &MessageHandler{
		deviceManager: e,
		logger:        logger,
		provisioner:   provisioner,
	}
}

type HandlerFunc func(ctx context.Context, msg *infra.Message) error

func (h *MessageHandler) HandleEvent(ctx context.Context, msg *infra.Message) error {
	// 1. Basic validity check
	if msg == nil || msg.Current == nil {
		h.logger.Warn("dropping config update: nil or missing current peer")
		return nil
	}

	h.logger.Debug("config update received",
		"version", msg.ConfigVersion,
		"incremental", msg.Changes != nil)

	// 2. Incremental processing logic (Fast Path)
	// Only perform granular device operations when Changes is non-nil and actually has changes
	if msg.Changes != nil && msg.Changes.HasChanges() {
		h.logger.Debug("applying incremental changes", "summary", msg.Changes.Summary())

		// --- Address and network changes ---
		if msg.Changes.AddressChanged {
			if msg.Current.Address == nil {
				// Case A: node lost its assigned IP, perform cleanup
				if len(msg.Changes.NetworkLeft) > 0 {
					h.logger.Warn("node left network, clearing IP and peer table")
					if err := h.provisioner.ApplyIP("remove", "", h.deviceManager.GetDeviceName()); err != nil {
						return fmt.Errorf("failed to remove IP: %w", err)
					}
					h.deviceManager.RemoveAllPeers()
				}
			} else {
				// Case B: new address assigned, force netmask to /32 (WireGuard standard)
				msg.Current.AllowedIPs = fmt.Sprintf("%s/32", *msg.Current.Address)
			}
		}

		// --- Key rotation (reserved logic) ---
		if msg.Changes.KeyChanged {
			h.logger.Info("WireGuard key rotation detected", "pub_key", msg.Current.PublicKey)
			// Trigger local key regeneration or update logic here
		}

		// --- Peer additions ---
		if len(msg.Changes.PeersAdded) > 0 {
			for _, peer := range msg.Changes.PeersAdded {
				// Strictly filter out self to prevent loopback or config conflicts
				if peer.PublicKey == msg.Current.PublicKey {
					continue
				}
				h.logger.Debug("adding peer", "peer_id", peer.PeerID, "endpoint", peer.Endpoint)
				if err := h.deviceManager.AddPeer(peer); err != nil {
					// Log error but do not abort; continue processing subsequent peers
					h.logger.Error("failed to add peer", err, "peer_id", peer.PeerID)
				}
			}
		}

		// --- Peer removals ---
		if len(msg.Changes.PeersRemoved) > 0 {
			for _, peer := range msg.Changes.PeersRemoved {
				h.logger.Debug("removing peer", "peer_id", peer.PeerID)
				if err := h.deviceManager.RemovePeer(peer); err != nil {
					h.logger.Error("failed to remove peer", err, "peer_id", peer.PeerID)
				}
			}
		}
	} else {
		// If Changes == nil, this is a full snapshot distribution
		h.logger.Debug("no incremental changes, falling back to full reconciliation")
	}

	// 3. Core exit: eventual consistency reconciliation (Safe Path)
	// Regardless of whether there were incremental changes, always call ApplyFullConfig.
	// This function should be idempotent: if kernel state already matches msg.Current, no writes are performed.
	if err := h.ApplyFullConfig(ctx, msg); err != nil {
		return fmt.Errorf("failed to apply full configuration: %w", err)
	}

	h.logger.Debug("config applied", "version", msg.ConfigVersion)
	return nil
}

// ApplyFullConfig when lattice start, apply full config
func (h *MessageHandler) ApplyFullConfig(ctx context.Context, msg *infra.Message) error {
	h.logger.Debug("reconciling full config", "version", msg.ConfigVersion)
	var err error

	// Set local IP (ConfigMap may not be ready at registration; address will be filled in by subsequent push)
	if msg.Current != nil && msg.Current.Address != nil {
		if err = h.provisioner.ApplyIP("add", *msg.Current.Address, h.deviceManager.GetDeviceName()); err != nil {
			h.logger.Error("failed to apply local IP", err, "addr", *msg.Current.Address)
			return err
		}
		// Write msg.Current (including server-assigned AllowedIPs) back to peerManager,
		// ensuring subsequent ICE offers carry the correct AllowedIPs in the Current field.
		if msg.Current.AllowedIPs == "" {
			msg.Current.AllowedIPs = fmt.Sprintf("%s/32", *msg.Current.Address)
		}
		if err = h.deviceManager.AddPeer(msg.Current); err != nil {
			h.logger.Error("failed to register local peer", err)
			return err
		}
	}

	// Apply remote peers
	if err = h.applyRemotePeers(ctx, msg); err != nil {
		h.logger.Error("failed to sync remote peers", err)
		return err
	}

	if err = h.applyFirewallRules(ctx, msg); err != nil {
		h.logger.Error("failed to apply firewall rules", err)
		return err
	}

	h.logger.Debug("full config reconciled", "version", msg.ConfigVersion)
	return nil
}

func (h *MessageHandler) applyRemotePeers(ctx context.Context, msg *infra.Message) error {
	for _, peer := range msg.ComputedPeers {
		// add peer to peers cached and probe start
		if err := h.deviceManager.AddPeer(peer); err != nil {
			return err
		}
	}
	return nil
}

func (h *MessageHandler) applyFirewallRules(ctx context.Context, msg *infra.Message) error {
	if msg.ComputedRules == nil {
		h.logger.Debug("no firewall rules in config, skipping provision")
		return nil
	}
	h.logger.Info("applying firewall rules",
		"enforcer", h.provisioner.Name(),
		"ingressRules", len(msg.ComputedRules.Ingress),
		"egressRules", len(msg.ComputedRules.Egress),
	)
	if err := h.provisioner.Provision(msg.ComputedRules); err != nil {
		h.logger.Error("firewall provision failed", err)
		return err
	}
	return nil
}
