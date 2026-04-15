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

//go:build !pro

// Package telemetry stubs out the Pro telemetry pipeline for community builds.
// VictoriaMetrics push is a Wireflow Pro feature.
package telemetry

import (
	"context"
	"errors"
	"time"

	"wireflow/internal/infra"
	"wireflow/internal/log"
)

var errProRequired = errors.New("telemetry push is a Wireflow Pro feature — upgrade at https://wireflow.run/pro")

// Labels is a map of Prometheus label name → value.
type Labels map[string]string

// Sample is a single (metric name, labels, value, timestamp) data point.
type Sample struct {
	Name        string
	Labels      Labels
	Value       float64
	TimestampMs int64
}

// NewSample is a convenience constructor.
func NewSample(name string, labels Labels, value float64, tsMs int64) Sample {
	return Sample{Name: name, Labels: labels, Value: value, TimestampMs: tsMs}
}

// Identity carries the local node's identifying labels.
type Identity struct {
	PeerID    string
	NetworkID string
	Interface string
}

// Scraper is the extension point for metric producers.
type Scraper interface {
	Name() string
	Scrape(ctx context.Context, id Identity, nowMs int64) ([]Sample, error)
}

// Config holds engine-level settings.
type Config struct {
	VMEndpoint string
	Interval   time.Duration
	MaxRetries int
}

// Collector stub — New always returns errProRequired so the collector is never started.
type Collector struct{}

func New(_ Config, _ *infra.PeerManager, _ *log.Logger, _ ...Scraper) (*Collector, error) {
	return nil, errProRequired
}

func (c *Collector) SetIdentity(_ Identity) {}

func (c *Collector) Run(_ context.Context) error { return nil }
