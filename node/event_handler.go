// Copyright 2025 Wireflow.io, Inc.
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

package node

import (
	"context"
	"fmt"
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/log"

	"k8s.io/klog/v2"
)

// event handler for node to handle event from management
type EventHandler struct {
	manager internal.EngineManager
	logger  *log.Logger
	client  *mgtclient.Client
}

func NewEventHandler(e internal.EngineManager, logger *log.Logger, client *mgtclient.Client) *EventHandler {
	return &EventHandler{
		manager: e,
		logger:  logger,
		client:  client,
	}
}

type HandlerFunc func(msg *internal.Message) error

func (handler *EventHandler) HandleEvent() HandlerFunc {
	return func(msg *internal.Message) error {
		handler.logger.Infof("Received config update %s: %s", msg.ConfigVersion, msg.Changes.Summary())
		if msg == nil {
			return nil
		}

		if msg.Changes == nil {
			return nil
		}

		if msg.Changes.HasChanges() {
			klog.Infof("Received remote changes: %v", msg)

			// 地址变化
			if msg.Changes.AddressChanged {
				if msg.Current.Address == "" {
					internal.SetDeviceIP()("remove", msg.Current.Address, handler.manager.GetWgConfiger().GetIfaceName())
				} else if msg.Current.Address != "" {
					internal.SetDeviceIP()("add", msg.Current.Address, handler.manager.GetWgConfiger().GetIfaceName())
				}
				msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", msg.Current.Address, 32)
				handler.manager.GetWgConfiger().GetPeersManager().AddPeer(msg.Current.PublicKey, msg.Current)
			}

			//
			if len(msg.Changes.NodesAdded) > 0 {
				handler.logger.Infof("nodes added: %v", msg.Changes.NodesAdded)
			}

			if len(msg.Changes.NodesRemoved) > 0 {
				handler.logger.Infof("nodes removed: %v", msg.Changes.NodesRemoved)
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

// ApplyFullConfig when node start, apply full config
func (handler *EventHandler) ApplyFullConfig(ctx context.Context, msg *internal.Message) error {
	logger := klog.FromContext(ctx)
	logger.Info("ApplyFullConfig", "message", msg)
	//apply nodes, add node to peers manager
	for _, node := range msg.Network.Nodes {
		handler.manager.GetWgConfiger().GetPeersManager().AddPeer(node.PublicKey, node)
		if err := handler.manager.AddNode(node); err != nil {
			return err
		}
	}

	// apply policies
	for _, policy := range msg.Network.Policies {
		logger.Info("ApplyPolicy", "policy", policy)
	}

	logger.V(4).Info("ApplyFullConfig done", "message", msg)
	return nil
}
