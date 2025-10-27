package probe

import (
	"context"
	"wireflow/internal"
	"wireflow/pkg/log"
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

func (d *drpChecker) ProbeConnect(ctx context.Context, isControlling bool, remoteOffer internal.Offer) error {
	return d.ProbeSuccess(ctx, d.drpAddr)
}

func (d *drpChecker) ProbeSuccess(ctx context.Context, addr string) error {
	return d.probe.ProbeSuccess(ctx, d.to, addr)
}

func (d *drpChecker) ProbeFailure(ctx context.Context, offer internal.Offer) error {
	return d.probe.ProbeFailed(ctx, d, offer)
}
