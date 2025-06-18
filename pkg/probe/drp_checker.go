package probe

import (
	"context"
	drpgrpc "linkany/drp/grpc"
	"linkany/internal"
	"linkany/pkg/log"
)

var (
	_ internal.Checker = (*drpChecker)(nil)
)

type drpChecker struct {
	probe   internal.Probe
	from    string
	to      string
	drpAddr string
	logger  *log.Logger
}

type DrpCheckerConfig struct {
	Probe   internal.Probe
	From    string
	To      string
	DrpAddr string // DRP address to connect to
}

func NewDrpChecker(cfg *DrpCheckerConfig) *drpChecker {
	return &drpChecker{
		probe:   cfg.Probe,
		from:    cfg.From,
		to:      cfg.To,
		drpAddr: cfg.DrpAddr,
		logger:  log.NewLogger(log.Loglevel, "drp-checker"),
	}
}

func (d *drpChecker) HandleOffer(offer internal.Offer) error {
	var err error
	switch offer.OfferType() {
	case internal.OfferTypeDrpOffer:
		// Handle DRP offer
		if err = d.probe.SendOffer(drpgrpc.MessageType_MessageDrpOfferAnswerType, d.from, d.to); err != nil {
			return err
		}

		return d.ProbeSuccess(d.to)
	case internal.OfferTypeDrpOfferAnswer:
		return d.ProbeSuccess(d.to)
	}
	return nil
}

func (d *drpChecker) ProbeConnect(ctx context.Context, isControlling bool, remoteOffer internal.Offer) error {
	return d.ProbeSuccess(d.drpAddr)
}

func (d *drpChecker) ProbeSuccess(addr string) error {
	return d.probe.ProbeSuccess(d.to, addr)
}

func (d *drpChecker) ProbeFailure(offer internal.Offer) error {
	return d.probe.ProbeFailed(d, offer)
}
