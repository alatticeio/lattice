package node

import (
	"fmt"
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/log"
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
		var err error
		h.logger.Infof("watch received event type: %v, node: %v", msg.EventType, msg.Current.String())
		switch msg.EventType {
		case internal.EventTypeLeaveNetwork:
			for _, node := range msg.Network.Nodes {
				wgConfigure := h.e.GetWgConfiger()
				if err := wgConfigure.RemovePeer(&internal.SetPeer{
					PublicKey: node.PublicKey,
					Remove:    true,
				}); err != nil {
					return err
				}

				//TODO add check when no same network peers exists, then delete the route.
				internal.SetRoute(h.logger)("delete", wgConfigure.GetAddress(), wgConfigure.GetIfaceName())
			}
		case internal.EventTypeJoinNetwork, internal.EventTypeNodeAdd:
			// update nodemanager
			msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", msg.Current.Address, 32)
			h.e.GetWgConfiger().GetPeersManager().AddPeer(msg.Current.PublicKey, msg.Current)
			for _, node := range msg.Network.Nodes {
				h.logger.Infof("received node data: %v", node.String())
				if err = h.client.AddPeer(node); err != nil {
					h.logger.Errorf("add node failed: %v", err)
				}
			}
		case internal.EventTypeIPChange:
			// 设置Device
			internal.SetDeviceIP()("add", msg.Current.Address, h.e.GetWgConfiger().GetIfaceName())
			// update nodemanager
			msg.Current.AllowedIPs = fmt.Sprintf("%s/%d", msg.Current.Address, 32)
			h.e.GetWgConfiger().GetPeersManager().AddPeer(msg.Current.PublicKey, msg.Current)
		case internal.EventTypeNodeRemove:
			// 设置Device
			internal.SetDeviceIP()("remove", msg.Current.Address, h.e.GetWgConfiger().GetIfaceName())
		case internal.EventTypeKeyChanged:
			if err := h.e.DeviceConfigure(&internal.DeviceConfig{
				PrivateKey: msg.Current.PrivateKey,
			}); err != nil {
				return err
			}
		case internal.EventTypeNodeUpdate:
		default:
			h.logger.Warningf("unknown event type: %v", msg.EventType)
		}

		return nil
	}
}
