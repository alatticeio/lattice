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

package client

import (
	"context"
	"fmt"
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
)

// event handler for wireflow to handle event from management
type EventHandler struct {
	deviceManager infra.Client
	logger        *log.Logger
	provisioner   infra.Provisioner
}

func NewEventHandler(e infra.Client, logger *log.Logger, provisioner infra.Provisioner) *EventHandler {
	return &EventHandler{
		deviceManager: e,
		logger:        logger,
		provisioner:   provisioner,
	}
}

type HandlerFunc func(msg *infra.Message) error

func (h *EventHandler) HandleEvent() HandlerFunc {
	return func(msg *infra.Message) error {
		if msg == nil {
			return nil
		}

		if msg.Changes == nil {
			return nil
		}
		h.logger.Infof("Received config update %s: %s", msg.ConfigVersion, msg.Changes.Summary())

		ctx := context.Background()
		if msg.Changes.HasChanges() {
			h.logger.Infof("Received remote changes: %v", msg)

			// 地址变化
			if msg.Changes.AddressChanged {
				if msg.Current.Address == nil {
					if len(msg.Changes.NetworkLeft) > 0 {
						//删除IP
						if err := h.provisioner.ApplyIP("remove", *msg.Current.Address, h.deviceManager.GetDeviceName()); err != nil {
							return err
						}
						//移除所有peers
						h.deviceManager.RemoveAllPeers()
					}

				} else if msg.Current.Address != nil {
					if err := h.provisioner.ApplyIP("add", *msg.Current.Address, h.deviceManager.GetDeviceName()); err != nil {
						return err
					}
				}
				msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", *msg.Current.Address, 32)
			}

			//reconfigure
			if msg.Changes.KeyChanged {
				//if err := h.deviceManager.SetupInterface(&infra.DeviceConfig{
				//	PrivateKey: msg.Current.PrivateKey,
				//}); err != nil {
				//	return err
				//}

				// TODO 重新连接所有的节点，基本不会发生，这要remove掉所有已连接的Peer, 然后重新连接
			}

			//
			if len(msg.Changes.PeersAdded) > 0 {
				h.logger.Infof("peers added: %v", msg.Changes.PeersAdded)
				for _, peer := range msg.Changes.PeersAdded {
					// add peer to peers cached
					if err := h.deviceManager.AddPeer(peer); err != nil {
						return err
					}
				}
			}

			// handle peer removed
			if len(msg.Changes.PeersRemoved) > 0 {
				h.logger.Infof("peers removed: %v", msg.Changes.PeersRemoved)
				for _, peer := range msg.Changes.PeersRemoved {
					if err := h.deviceManager.RemovePeer(peer); err != nil {
						return err
					}
				}
			}

		}

		return h.ApplyFullConfig(ctx, msg)
	}
}

// ApplyFullConfig when wireflow start, apply full config
func (h *EventHandler) ApplyFullConfig(ctx context.Context, msg *infra.Message) error {
	h.logger.Verbosef("ApplyFullConfig start: %v", msg)
	var err error

	//设置Peers
	if err = h.applyRemotePeers(ctx, msg); err != nil {
		h.logger.Errorf("ApplyFullConfig err: %v", err)
		return err
	}

	if err = h.applyFirewallRules(ctx, msg); err != nil {
		h.logger.Errorf("ApplyFullConfig err: %v", err)
		return err
	}

	h.logger.Verbosef("ApplyFullConfig done, message version: %v", msg.ConfigVersion)
	return nil
}

func (h *EventHandler) applyRemotePeers(ctx context.Context, msg *infra.Message) error {
	for _, peer := range msg.ComputedPeers {
		// add peer to peers cached
		//h.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(peer.PublicKey, peer)
		h.deviceManager.AddPeer(peer)
		if err := h.deviceManager.AddPeer(peer); err != nil {
			return err
		}
	}
	return nil
}

func (h *EventHandler) applyFirewallRules(ctx context.Context, msg *infra.Message) error {
	if msg.ComputedRules == nil {
		return nil
	}
	var err error
	ingress := msg.ComputedRules.IngressRules
	egress := msg.ComputedRules.EgressRules

	for _, rule := range ingress {
		if err = h.provisioner.ApplyRule("add", rule); err != nil {
			return err
		}
	}

	for _, rule := range egress {
		if err = h.provisioner.ApplyRule("add", rule); err != nil {
			return err
		}
	}
	return nil
}
