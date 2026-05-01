package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryRequest(t *testing.T) {
	req := &QueryRequest{
		MetricType: "online_count",
		Namespace:  "wf-test",
		TimeRange:  TimeRange{Lookback: "5m"},
	}
	assert.Equal(t, "online_count", req.MetricType)
}

func TestQueryRangeRequest(t *testing.T) {
	now := time.Now()
	req := &QueryRangeRequest{
		MetricType: "tx_trend",
		Namespace:  "wf-test",
		Start:      now.Add(-time.Hour),
		End:        now,
		Step:       2 * time.Minute,
	}
	assert.Equal(t, 2*time.Minute, req.Step)
}

func TestResultTypes(t *testing.T) {
	assert.Equal(t, ResultType("instant"), ResultInstant)
	assert.Equal(t, ResultType("range"), ResultRange)
}

func TestMockAdapter_Defaults(t *testing.T) {
	m := &MockAdapter{}
	assert.Equal(t, "mock", m.Name())
	_, err := m.Query(context.Background(), &QueryRequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMetricNotFound)
}
