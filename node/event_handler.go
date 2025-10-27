package node

import (
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/log"
)

// event handler for node to handle event from management
type EventHandler struct {
	engine internal.EngineManager
	logger *log.Logger
	client *mgtclient.Client
}

func NewEventHandler(engine internal.EngineManager, logger *log.Logger, client *mgtclient.Client) *EventHandler {
	return &EventHandler{
		engine: engine,
		logger: logger,
		client: client,
	}
}

type HandlerFunc func(msg *internal.Message) error

func (h *EventHandler) HandleEvent() HandlerFunc {
	return func(msg *internal.Message) error {
		var err error
		switch msg.EventType {
		case internal.EventTypeLeaveNetwork:
			for _, node := range msg.Network.Nodes {
				h.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeLeaveNetwork, node.String())
				wgConfigure := h.engine.GetWgConfiger()
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
			for _, node := range msg.Network.Nodes {
				h.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeJoinNetwork, node.String())
				if err = h.client.AddPeer(node); err != nil {
					h.logger.Errorf("add node failed: %v", err)
				}
			}
		case internal.EventTypeIPChange:
			h.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeIPChange, msg.Current.String())
			// 设置Device
			internal.SetDeviceIP()("add", msg.Current.Address, h.engine.GetWgConfiger().GetIfaceName())

			if err = h.engine.DeviceConfigure(&internal.DeviceConfig{
				PrivateKey: msg.Current.PrivateKey,
			}); err != nil {
				return err
			}
		case internal.EventTypeNodeRemove:
			h.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeNodeRemove, msg.Current.String())
		case internal.EventTypeNodeUpdate:
			h.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeNodeUpdate, msg.Current.String())
		default:
			h.logger.Warningf("unknown event type: %v", msg.EventType)
		}

		return nil
	}
}
