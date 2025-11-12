package node

import (
	"fmt"
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/log"

	"k8s.io/klog/v2"
)

// event handler for node to handle event from management
type EventHandler struct {
	e      internal.EngineManager
	logger *log.Logger
	client *mgtclient.Client
}

func NewEventHandler(e internal.EngineManager, logger *log.Logger, client *mgtclient.Client) *EventHandler {
	return &EventHandler{
		e:      e,
		logger: logger,
		client: client,
	}
}

type HandlerFunc func(msg *internal.Message) error

func (h *EventHandler) HandleEvent() HandlerFunc {
	return func(msg *internal.Message) error {
		h.logger.Infof("Received config update %s: %s", msg.ConfigVersion, msg.Changes.Summary())
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
					internal.SetDeviceIP()("remove", msg.Current.Address, h.e.GetWgConfiger().GetIfaceName())
				} else if msg.Current.Address != "" {
					internal.SetDeviceIP()("add", msg.Current.Address, h.e.GetWgConfiger().GetIfaceName())
				}
				msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", msg.Current.Address, 32)
				h.e.GetWgConfiger().GetPeersManager().AddPeer(msg.Current.PublicKey, msg.Current)
			}

			//
			if len(msg.Changes.NodesAdded) > 0 {
				h.logger.Infof("nodes added: %v", msg.Changes.NodesAdded)
			}

			if len(msg.Changes.NodesRemoved) > 0 {
				h.logger.Infof("nodes removed: %v", msg.Changes.NodesRemoved)
			}

			if len(msg.Changes.PoliciesAdded) > 0 {
				h.logger.Infof("policies added: %v", msg.Changes.PoliciesAdded)
			}

			if len(msg.Changes.PoliciesUpdated) > 0 {
				h.logger.Infof("policies updated: %v", msg.Changes.PoliciesUpdated)
			}

		}

		return nil
	}
}

func applyFullConfig(config *internal.Message) error {
	return nil
}
