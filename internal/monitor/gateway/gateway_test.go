package gateway

import (
	"context"
	"testing"

	"github.com/alatticeio/lattice/internal/monitor/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitorGateway(t *testing.T) {
	g := NewMonitorGateway()
	assert.NotNil(t, g)
}

func TestRegister(t *testing.T) {
	g := NewMonitorGateway()
	g.Register(&adapter.MockAdapter{NameVal: "test"})
	assert.Len(t, g.adapters, 1)
}

func TestResolveAdapter_VMDefault(t *testing.T) {
	g := NewMonitorGateway()
	g.Register(&adapter.MockAdapter{NameVal: "victoriametrics"})
	a, err := g.resolveAdapter("some_metric")
	require.NoError(t, err)
	assert.Equal(t, "victoriametrics", a.Name())
}

func TestResolveAdapter_HeartbeatPrefix(t *testing.T) {
	g := NewMonitorGateway()
	g.Register(&adapter.MockAdapter{NameVal: "heartbeat"})
	a, err := g.resolveAdapter("heartbeat_online_count")
	require.NoError(t, err)
	assert.Equal(t, "heartbeat", a.Name())
}

func TestResolveAdapter_NoAdapter(t *testing.T) {
	g := NewMonitorGateway()
	_, err := g.resolveAdapter("some_metric")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no adapter available")
}

func TestHealth_Empty(t *testing.T) {
	g := NewMonitorGateway()
	err := g.Health(context.Background())
	assert.NoError(t, err)
}
