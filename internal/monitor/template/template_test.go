package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry_LoadsDefaults(t *testing.T) {
	r, err := NewRegistry()
	require.NoError(t, err)
	assert.NotEmpty(t, r.templates)
	_, err = r.Get("online_count")
	require.NoError(t, err)
}

func TestGet_NotFound(t *testing.T) {
	r, err := NewRegistry()
	require.NoError(t, err)
	_, err = r.Get("nonexistent_metric")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestRender_OnlineCount(t *testing.T) {
	r, err := NewRegistry()
	require.NoError(t, err)
	promql, err := r.Render("online_count", map[string]any{"Namespace": "wf-abc123"})
	require.NoError(t, err)
	assert.Contains(t, promql, `network_id="wf-abc123"`)
	assert.Contains(t, promql, "lattice_node_uptime_seconds")
}

func TestRender_Throughput(t *testing.T) {
	r, err := NewRegistry()
	require.NoError(t, err)
	promql, err := r.Render("tx_throughput", map[string]any{"Namespace": "wf-test"})
	require.NoError(t, err)
	assert.Contains(t, promql, "lattice_node_traffic_bytes_total")
	assert.Contains(t, promql, `direction="tx"`)
}
