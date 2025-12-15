// Copyright 2025 The Wireflow Authors, Inc.
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

package probe

import (
	"context"
	"wireflow/internal/core/domain"
	"wireflow/pkg/log"
)

var (
	_ domain.Checker = (*drpChecker)(nil)
)

type drpChecker struct {
	probe   domain.Probe
	from    string
	to      string
	drpAddr string
	logger  *log.Logger
}

type DrpCheckerConfig struct {
	Probe   domain.Probe
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

func (d *drpChecker) ProbeConnect(ctx context.Context, isControlling bool, remoteOffer domain.Offer) error {
	return d.ProbeSuccess(ctx, d.drpAddr)
}

func (d *drpChecker) ProbeSuccess(ctx context.Context, addr string) error {
	return d.probe.ProbeSuccess(ctx, d.to, addr)
}

func (d *drpChecker) ProbeFailure(ctx context.Context, offer domain.Offer) error {
	return d.probe.ProbeFailed(ctx, d, offer)
}
