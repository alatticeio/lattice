package adapter

import (
	"context"
	"fmt"
	"time"
)

var ErrMetricNotFound = fmt.Errorf("metric not found")

// Adapter is the unified interface for all monitoring data sources.
type Adapter interface {
	Name() string
	Health(ctx context.Context) error
	Query(ctx context.Context, req *QueryRequest) (*QueryResult, error)
	QueryRange(ctx context.Context, req *QueryRangeRequest) (*QueryRangeResult, error)
}

type QueryRequest struct {
	MetricType string
	Labels     map[string]string
	Namespace  string
	TimeRange  TimeRange
}

type QueryRangeRequest struct {
	MetricType string
	Labels     map[string]string
	Namespace  string
	Start      time.Time
	End        time.Time
	Step       time.Duration
}

type QueryResult struct {
	Type   ResultType
	Series []Series
	Scalar *ScalarResult
	Table  []map[string]any
}

type QueryRangeResult struct {
	Series []Series
}

type Series struct {
	Labels map[string]string `json:"labels"`
	Values []DataPoint       `json:"values"`
}

type DataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type ScalarResult struct {
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
}

type TimeRange struct {
	Start    time.Time
	End      time.Time
	Lookback string
}

type ResultType string

const (
	ResultInstant ResultType = "instant"
	ResultRange   ResultType = "range"
	ResultTable   ResultType = "table"
	ResultScalar  ResultType = "scalar"
)

// MockAdapter for testing.
type MockAdapter struct {
	QueryFn      func(ctx context.Context, req *QueryRequest) (*QueryResult, error)
	QueryRangeFn func(ctx context.Context, req *QueryRangeRequest) (*QueryRangeResult, error)
	HealthFn     func(ctx context.Context) error
	NameVal      string
}

func (m *MockAdapter) Name() string {
	if m.NameVal != "" {
		return m.NameVal
	}
	return "mock"
}

func (m *MockAdapter) Health(ctx context.Context) error {
	if m.HealthFn != nil {
		return m.HealthFn(ctx)
	}
	return nil
}

func (m *MockAdapter) Query(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	if m.QueryFn != nil {
		return m.QueryFn(ctx, req)
	}
	return nil, ErrMetricNotFound
}

func (m *MockAdapter) QueryRange(ctx context.Context, req *QueryRangeRequest) (*QueryRangeResult, error) {
	if m.QueryRangeFn != nil {
		return m.QueryRangeFn(ctx, req)
	}
	return nil, ErrMetricNotFound
}
