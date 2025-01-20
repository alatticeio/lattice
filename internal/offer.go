package internal

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

type Offer interface {
	Marshal() (int, []byte, error)
}

type OfferManager interface {
	SendOffer(FrameType, wgtypes.Key, wgtypes.Key, Offer) error
	ReceiveOffer() (Offer, error)
}
