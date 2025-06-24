package internal

import (
	"context"
	"encoding/json"
	"fmt"
	drpgrpc "linkany/drp/grpc"
)

type Offer interface {
	Marshal() (int, []byte, error)
	OfferType() OfferType
	TieBreaker() uint64
	GetNode() *NodeMessage
}

type OfferHandler interface {
	SendOffer(context.Context, drpgrpc.MessageType, string, string, Offer) error
	ReceiveOffer(ctx context.Context, message *drpgrpc.DrpMessage) error
}

type OfferType int

const (
	OfferTypeDrpOffer OfferType = iota
	OfferTypeDrpOfferAnswer
	OfferTypeDirectOffer
	OfferTypeDirectOfferAnswer
	OfferTypeRelayOffer
	OfferTypeRelayAnswer
)

type ConnectType int

const (
	DirectType ConnectType = iota
	RelayType
	DrpType
)

func (s ConnectType) String() string {
	switch s {
	case DirectType:
		return "direct"
	case RelayType:
		return "relay"
	case DrpType:
		return "drp"
	default:
		return "unknown"
	}
}

func (s ConnectType) MarshalJSON() ([]byte, error) {
	// 将枚举值转换为字符串
	return json.Marshal(s.String())
}

var statusMap = map[string]ConnectType{
	"direct": DirectType,
	"drp":    DrpType,
	"relay":  RelayType,
}

func (s *ConnectType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// 根据字符串设置Status值
	if status, ok := statusMap[str]; ok {
		*s = status
		return nil
	}

	return fmt.Errorf("invalid status: %s", str)
}

func UnmarshalOffer[T Offer](data []byte, t T) (T, error) {
	err := json.Unmarshal(data, t)
	if err != nil {
		var zero T
		return zero, err
	}
	return t, nil
}
