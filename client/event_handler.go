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
	"wireflow/internal/core/domain"
	"wireflow/internal/core/infra"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/log"
)

// event handler for wireflow to handle event from management
type EventHandler struct {
	deviceManager domain.IClient
	logger        *log.Logger
	client        *mgtclient.Client
}

func NewEventHandler(e domain.IClient, logger *log.Logger, client *mgtclient.Client) *EventHandler {
	return &EventHandler{
		deviceManager: e,
		logger:        logger,
		client:        client,
	}
}

type HandlerFunc func(msg *domain.Message) error

func (handler *EventHandler) HandleEvent() HandlerFunc {
	return func(msg *domain.Message) error {
		if msg == nil {
			return nil
		}

		if msg.Changes == nil {
			return nil
		}
		handler.logger.Infof("Received config update %s: %s", msg.ConfigVersion, msg.Changes.Summary())

		if msg.Changes.HasChanges() {
			handler.logger.Infof("Received remote changes: %v", msg)

			// 地址变化
			if msg.Changes.AddressChanged {
				if msg.Current.Address == "" {
					if len(msg.Changes.NetworkLeft) > 0 {
						//删除IP
						infra.SetDeviceIP()("remove", msg.Current.Address, handler.deviceManager.GetDeviceConfiger().GetIfaceName())
						//移除所有peers
						handler.deviceManager.RemoveAllPeers()
					}

				} else if msg.Current.Address != "" {
					infra.SetDeviceIP()("add", msg.Current.Address, handler.deviceManager.GetDeviceConfiger().GetIfaceName())
				}
				msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", msg.Current.Address, 32)
				handler.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(msg.Current.PublicKey, msg.Current)
			}

			//reconfigure
			if msg.Changes.KeyChanged {
				if err := handler.deviceManager.Configure(&domain.DeviceConfig{
					PrivateKey: msg.Current.PrivateKey,
				}); err != nil {
					return err
				}

				// TODO 重新连接所有的节点
			}

			//
			if len(msg.Changes.PeersAdded) > 0 {
				handler.logger.Infof("peers added: %v", msg.Changes.PeersAdded)
				for _, peer := range msg.Changes.PeersAdded {
					// add peer to peers cached
					handler.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(peer.PublicKey, peer)
					if err := handler.deviceManager.AddPeer(peer); err != nil {
						return err
					}
				}
			}

			// handle peer removed
			if len(msg.Changes.PeersRemoved) > 0 {
				handler.logger.Infof("peers removed: %v", msg.Changes.PeersRemoved)
				for _, peer := range msg.Changes.PeersRemoved {
					if err := handler.deviceManager.RemovePeer(peer); err != nil {
						return err
					}
				}
			}

			if len(msg.Changes.PoliciesAdded) > 0 || len(msg.Changes.PoliciesRemoved) > 0 || len(msg.Changes.PoliciesUpdated) > 0 {
				peers := handler.handlePeerFromPolicy(msg.Network, msg.Changes.PoliciesAdded)
				for _, peer := range peers {
					handler.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(peer.PublicKey, peer)
					if err := handler.deviceManager.AddPeer(peer); err != nil {
						return err
					}
				}
			}

		}

		return nil
	}
}

// ApplyFullConfig when wireflow start, apply full config
func (handler *EventHandler) ApplyFullConfig(ctx context.Context, msg *domain.Message) error {
	handler.logger.Verbosef("ApplyFullConfig start: %v", msg)

	peers := handler.handlePeerFromPolicy(msg.Network, msg.Policies)
	//apply peers, add peer to peers deviceManager
	for _, peer := range peers {
		handler.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(peer.PublicKey, peer)
		if err := handler.deviceManager.AddPeer(peer); err != nil {
			return err
		}
	}

	handler.logger.Verbosef("ApplyFullConfig done, message version: %v", msg.ConfigVersion)
	return nil
}

// handlePeerFromPolicy when Network peers is not nil and also policy's peers  is not nil, should filter peers
// return filtered peers added, removed
func (handler *EventHandler) handlePeerFromPolicy(network *domain.Network, policies []*domain.Policy) []*domain.Peer {
	networkPeers := network.Peers
	if networkPeers == nil {
		return nil
	}

	netPeerSet := peerToSet(networkPeers)
	for _, policy := range policies {
		ingresses := policy.Ingress
		egresses := policy.Egress

		for _, egress := range egresses {
			for _, peer := range egress.Peers {
				if _, ok := netPeerSet[peer.PublicKey]; !ok {
					delete(netPeerSet, peer.PublicKey)
				}
			}
		}

		for _, ingress := range ingresses {
			for _, peer := range ingress.Peers {
				if _, ok := netPeerSet[peer.PublicKey]; !ok {
					delete(netPeerSet, peer.PublicKey)
				}
			}
		}

	}

	for _, peer := range networkPeers {
		if _, ok := netPeerSet[peer.PublicKey]; !ok {
			delete(netPeerSet, peer.PublicKey)
		}
	}

	return networkPeers
}

func peerToSet(peers []*domain.Peer) map[string]struct{} {
	m := make(map[string]struct{})
	for _, peer := range peers {
		m[peer.PublicKey] = struct{}{}
	}

	return m
}
