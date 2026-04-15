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

//go:build pro

// Package telemetry provides a lightweight, push-only metric pipeline for wireflow-agent.
//
// # Architecture
//
//	┌──────────────────────────────────────────────────────┐
//	│                    Collector (engine)                │
//	│                                                      │
//	│  ┌────────────┐  []Sample  ┌──────────┐  []byte  ┌──────┐ │
//	│  │ Scraper[]  │ ─────────▶ │ Encoder  │ ────────▶ │ Push │ │
//	│  └────────────┘            └──────────┘           └──────┘ │
//	└──────────────────────────────────────────────────────┘
//
// Built-in Scrapers:
//   - SystemScraper    CPU / memory / goroutines
//   - WireGuardScraper per-peer traffic, status, handshake; node & workspace totals; peering traffic
//   - ICMPScraper      latency (ms) + packet-loss (%) via concurrent ICMP probes
//
// # Extending
//
// Implement the Scraper interface and pass your implementation to New():
//
//	type MyMetrics struct{}
//	func (m *MyMetrics) Name() string { return "my_metrics" }
//	func (m *MyMetrics) Scrape(ctx context.Context, id Identity, nowMs int64) ([]Sample, error) {
//	    return []Sample{
//	        NewSample("wireflow_my_gauge",
//	            Labels{"peer_id": id.PeerID, "network_id": id.NetworkID},
//	            42.0, nowMs),
//	    }, nil
//	}
//	// Pass as a variadic argument:
//	telemetry.New(cfg, peers, logger, &MyMetrics{})
//
// Compatible with CGO_ENABLED=0 static builds.
package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"

	victoriametrics "github.com/VictoriaMetrics/metrics"
	"github.com/klauspost/compress/s2"
	"google.golang.org/protobuf/encoding/protowire"

	"wireflow/internal/infra"
	"wireflow/internal/log"
)

// ─── Public extension API ─────────────────────────────────────────────────────

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

// Identity carries the local node's identifying labels, passed to every Scraper
// on each cycle so scrapers do not need to store identity themselves.
type Identity struct {
	PeerID    string // AppId of this node
	NetworkID string // Workspace/network this node belongs to
	Interface string // WireGuard interface name (e.g. "wg0")
}

// Scraper is the extension point for metric producers.
// Every call to Scrape must be idempotent and safe for repeated calls.
type Scraper interface {
	// Name returns a short identifier used in log messages.
	Name() string
	// Scrape collects metrics and returns them as a slice of Samples.
	// nowMs is Unix time in milliseconds, shared across all scrapers in one cycle.
	// Returning a non-nil error causes a warning log but does not stop other scrapers.
	Scrape(ctx context.Context, id Identity, nowMs int64) ([]Sample, error)
}

// ─── Engine config ────────────────────────────────────────────────────────────

// Config holds engine-level settings.
// Scraper-specific tuning lives in each scraper's own constructor.
type Config struct {
	// VMEndpoint is the VictoriaMetrics remote write URL, e.g. "http://vm:8428/api/v1/write".
	// An empty string disables push while scrapers still run (useful for testing).
	VMEndpoint string
	// Interval between full scrape+push cycles. Defaults to 30 s.
	Interval time.Duration
	// MaxRetries is the number of HTTP push attempts on transient failure. Defaults to 3.
	MaxRetries int
}

func (c *Config) setDefaults() {
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
}

// ─── Collector (engine) ───────────────────────────────────────────────────────

// Collector drives all registered Scrapers on a fixed interval, encodes results
// as Prometheus Remote Write (protobuf + Snappy), and pushes to VictoriaMetrics.
type Collector struct {
	cfg      Config
	id       Identity
	log      *log.Logger
	scrapers []Scraper
	client   *http.Client
	// set exposes the VM metrics registry; currently used for node-level gauges
	// that can serve a future scrape endpoint alongside push mode.
	set *victoriametrics.Set
}

// New creates a Collector with the three built-in scrapers (system, wireguard, icmp)
// plus any additional Scraper implementations provided via extra.
func New(cfg Config, peers *infra.PeerManager, logger *log.Logger, extra ...Scraper) (*Collector, error) {
	cfg.setDefaults()

	wgScraper, err := NewWireGuardScraper(peers)
	if err != nil {
		return nil, fmt.Errorf("telemetry: wireguard scraper init: %w", err)
	}

	c := &Collector{
		cfg:    cfg,
		log:    logger,
		client: &http.Client{Timeout: 10 * time.Second},
		set:    victoriametrics.NewSet(),
	}
	c.scrapers = append(c.scrapers,
		NewSystemScraper(),
		wgScraper,
		NewICMPScraper(peers, 3, 2*time.Second),
	)
	c.scrapers = append(c.scrapers, extra...)
	return c, nil
}

// SetIdentity updates the local node identity. Call this after the agent receives
// its first network map if NetworkID was not yet available at construction time.
func (c *Collector) SetIdentity(id Identity) { c.id = id }

// Run starts the collection and push loop. Blocks until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	names := make([]string, len(c.scrapers))
	for i, s := range c.scrapers {
		names[i] = s.Name()
	}
	c.log.Info("telemetry collector started",
		"endpoint", c.cfg.VMEndpoint,
		"interval", c.cfg.Interval,
		"scrapers", names,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("telemetry collector stopped")
			return nil
		case <-ticker.C:
			if err := c.tick(ctx); err != nil {
				c.log.Warn("telemetry cycle failed", "err", err)
			}
		}
	}
}

func (c *Collector) tick(ctx context.Context) error {
	nowMs := time.Now().UnixMilli()
	var all []Sample

	for _, sc := range c.scrapers {
		samples, err := sc.Scrape(ctx, c.id, nowMs)
		if err != nil {
			c.log.Warn("scraper error", "scraper", sc.Name(), "err", err)
			continue // non-fatal; other scrapers proceed
		}
		all = append(all, samples...)
	}

	if len(all) == 0 || c.cfg.VMEndpoint == "" {
		return nil
	}

	payload, err := encodeRemoteWrite(all)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return c.pushWithRetry(ctx, payload)
}

// WritePrometheus writes the VM metrics Set in Prometheus text format.
func (c *Collector) WritePrometheus(w io.Writer) { c.set.WritePrometheus(w) }

// ─── Remote Write encoding ────────────────────────────────────────────────────
//
// Proto3 schema (field numbers per prometheus/prompb):
//
//	WriteRequest  { repeated TimeSeries timeseries = 1 }
//	TimeSeries    { repeated Label labels = 1; repeated Sample samples = 2 }
//	Label         { string name = 1; string value = 2 }
//	Sample        { double value = 1; int64 timestamp_ms = 2 }

func encodeRemoteWrite(samples []Sample) ([]byte, error) {
	var writeReq []byte

	for _, s := range samples {
		var tsBuf []byte

		// Build sorted label set with __name__ prepended.
		type kv struct{ k, v string }
		lbls := make([]kv, 0, len(s.Labels)+1)
		lbls = append(lbls, kv{"__name__", s.Name})
		for k, v := range s.Labels {
			lbls = append(lbls, kv{k, v})
		}
		sort.Slice(lbls, func(i, j int) bool { return lbls[i].k < lbls[j].k })

		for _, lp := range lbls {
			var lBuf []byte
			lBuf = protowire.AppendTag(lBuf, 1, protowire.BytesType)
			lBuf = protowire.AppendString(lBuf, lp.k)
			lBuf = protowire.AppendTag(lBuf, 2, protowire.BytesType)
			lBuf = protowire.AppendString(lBuf, lp.v)
			tsBuf = protowire.AppendTag(tsBuf, 1, protowire.BytesType)
			tsBuf = protowire.AppendBytes(tsBuf, lBuf)
		}

		// Sample: double value (field 1, wire type 1) + int64 timestamp (field 2, varint).
		var sBuf []byte
		sBuf = protowire.AppendTag(sBuf, 1, protowire.Fixed64Type)
		sBuf = protowire.AppendFixed64(sBuf, math.Float64bits(s.Value))
		sBuf = protowire.AppendTag(sBuf, 2, protowire.VarintType)
		sBuf = protowire.AppendVarint(sBuf, uint64(s.TimestampMs))
		tsBuf = protowire.AppendTag(tsBuf, 2, protowire.BytesType)
		tsBuf = protowire.AppendBytes(tsBuf, sBuf)

		writeReq = protowire.AppendTag(writeReq, 1, protowire.BytesType)
		writeReq = protowire.AppendBytes(writeReq, tsBuf)
	}

	// s2.EncodeSnappy produces standard Snappy block format, compatible with
	// github.com/golang/snappy used by VictoriaMetrics.
	return s2.EncodeSnappy(nil, writeReq), nil
}

// ─── HTTP push ────────────────────────────────────────────────────────────────

func (c *Collector) pushWithRetry(ctx context.Context, payload []byte) error {
	var lastErr error
	for i := 0; i < c.cfg.MaxRetries; i++ {
		if err := c.push(ctx, payload); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<uint(i)) * time.Second):
				continue
			}
		}
		return nil
	}
	return fmt.Errorf("push failed after %d retries: %w", c.cfg.MaxRetries, lastErr)
}

func (c *Collector) push(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.VMEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("vm returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ─── Shared helpers (used by scrapers) ───────────────────────────────────────

// mergeLabels returns a new Labels that is the union of base and extra.
// Keys in extra take precedence on collision.
func mergeLabels(base, extra Labels) Labels {
	out := make(Labels, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
