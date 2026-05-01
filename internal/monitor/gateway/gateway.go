package gateway

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/alatticeio/lattice/internal/monitor/adapter"
)

// MonitorGateway routes metric queries to the appropriate adapter.
type MonitorGateway struct {
	adapters map[string]adapter.Adapter
	mu       sync.RWMutex
}

// NewMonitorGateway creates a new empty gateway.
func NewMonitorGateway() *MonitorGateway {
	return &MonitorGateway{adapters: make(map[string]adapter.Adapter)}
}

// Register adds an adapter to the gateway.
func (g *MonitorGateway) Register(a adapter.Adapter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.adapters[a.Name()] = a
}

// Query routes an instant query to the appropriate adapter.
func (g *MonitorGateway) Query(ctx context.Context, req *adapter.QueryRequest) (*adapter.QueryResult, error) {
	a, err := g.resolveAdapter(req.MetricType)
	if err != nil {
		return nil, err
	}
	return a.Query(ctx, req)
}

// QueryRange routes a range query to the appropriate adapter.
func (g *MonitorGateway) QueryRange(ctx context.Context, req *adapter.QueryRangeRequest) (*adapter.QueryRangeResult, error) {
	a, err := g.resolveAdapter(req.MetricType)
	if err != nil {
		return nil, err
	}
	return a.QueryRange(ctx, req)
}

// Health checks all registered adapters.
func (g *MonitorGateway) Health(ctx context.Context) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for name, a := range g.adapters {
		if err := a.Health(ctx); err != nil {
			return fmt.Errorf("adapter %s unhealthy: %w", name, err)
		}
	}
	return nil
}

func (g *MonitorGateway) resolveAdapter(metricType string) (adapter.Adapter, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if strings.HasPrefix(metricType, "heartbeat_") {
		if a, ok := g.adapters["heartbeat"]; ok {
			return a, nil
		}
	}
	if a, ok := g.adapters["victoriametrics"]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("no adapter available for metric: %s", metricType)
}
