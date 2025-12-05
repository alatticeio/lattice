// Copyright 2025 wireflowio.com, Inc.
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
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/log"
)

// event handler for wireflow to handle event from management
type EventHandler struct {
	deviceManager internal.IClient
	logger        *log.Logger
	client        *mgtclient.Client
}

func NewEventHandler(e internal.IClient, logger *log.Logger, client *mgtclient.Client) *EventHandler {
	return &EventHandler{
		deviceManager: e,
		logger:        logger,
		client:        client,
	}
}

type HandlerFunc func(msg *internal.Message) error

func (handler *EventHandler) HandleEvent() HandlerFunc {
	return func(msg *internal.Message) error {
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
						internal.SetDeviceIP()("remove", msg.Current.Address, handler.deviceManager.GetDeviceConfiger().GetIfaceName())
						//移除所有peers
						handler.deviceManager.RemoveAllPeers()
					}

				} else if msg.Current.Address != "" {
					internal.SetDeviceIP()("add", msg.Current.Address, handler.deviceManager.GetDeviceConfiger().GetIfaceName())
				}
				msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", msg.Current.Address, 32)
				handler.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(msg.Current.PublicKey, msg.Current)
			}

			//reconfigure
			if msg.Changes.KeyChanged {
				if err := handler.deviceManager.Configure(&internal.DeviceConfig{
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

			if len(msg.Changes.PoliciesAdded) > 0 {
				handler.logger.Infof("policies added: %v", msg.Changes.PoliciesAdded)
			}

			if len(msg.Changes.PoliciesUpdated) > 0 {
				handler.logger.Infof("policies updated: %v", msg.Changes.PoliciesUpdated)
			}

		}

		return nil
	}
}

// ApplyFullConfig when wireflow start, apply full config
func (handler *EventHandler) ApplyFullConfig(ctx context.Context, msg *internal.Message) error {
	handler.logger.Verbosef("ApplyFullConfig start: %v", msg)
	//apply peers, add peer to peers deviceManager
	for _, peer := range msg.Network.Peers {
		handler.deviceManager.GetDeviceConfiger().GetPeersManager().AddPeer(peer.PublicKey, peer)
		if err := handler.deviceManager.AddPeer(peer); err != nil {
			return err
		}
	}

	// apply policies
	for _, policy := range msg.Network.Policies {
		handler.logger.Verbosef("ApplyPolicy: %v", policy)
	}

	handler.logger.Verbosef("ApplyFullConfig done, message version: %v", msg.ConfigVersion)
	return nil
}
