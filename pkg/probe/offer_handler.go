package probe

import (
	"linkany/internal"
)

type OfferHandler interface {
	Handle(offer internal.Offer) error
}
