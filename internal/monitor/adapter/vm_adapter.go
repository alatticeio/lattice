package adapter

import (
	"context"
	"fmt"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/alatticeio/lattice/internal/monitor/template"
)

// VMAdapter implements Adapter using VictoriaMetrics PromQL API.
type VMAdapter struct {
	api       v1.API
	baseURL   string
	templates *template.TemplateRegistry
}

// NewVMAdapter creates a new VMAdapter.
func NewVMAdapter(baseURL string, templates *template.TemplateRegistry) (*VMAdapter, error) {
	client, err := promapi.NewClient(promapi.Config{Address: baseURL})
	if err != nil {
		return nil, fmt.Errorf("create prometheus client: %w", err)
	}
	return &VMAdapter{
		api:       v1.NewAPI(client),
		baseURL:   baseURL,
		templates: templates,
	}, nil
}

func (a *VMAdapter) Name() string { return "victoriametrics" }

func (a *VMAdapter) Health(ctx context.Context) error {
	_, _, err := a.api.Query(ctx, "1+1", time.Now())
	return err
}

func (a *VMAdapter) Query(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	tpl, err := a.templates.Get(req.MetricType)
	if err != nil {
		return nil, err
	}

	params := map[string]any{"Namespace": req.Namespace}
	for k, v := range req.Labels {
		params[k] = v
	}
	promql, err := a.templates.Render(req.MetricType, params)
	if err != nil {
		return nil, err
	}

	ts := req.TimeRange.End
	if ts.IsZero() {
		ts = time.Now()
	}
	val, _, err := a.api.Query(ctx, promql, ts)
	if err != nil {
		return nil, fmt.Errorf("VM query error: %w", err)
	}

	return parseVMValue(val, tpl.ResultType)
}

func (a *VMAdapter) QueryRange(ctx context.Context, req *QueryRangeRequest) (*QueryRangeResult, error) {
	_, err := a.templates.Get(req.MetricType)
	if err != nil {
		return nil, err
	}

	params := map[string]any{
		"Namespace": req.Namespace,
		"Step":      req.Step.String(),
	}
	for k, v := range req.Labels {
		params[k] = v
	}
	promql, err := a.templates.Render(req.MetricType, params)
	if err != nil {
		return nil, err
	}

	r := v1.Range{Start: req.Start, End: req.End, Step: req.Step}
	val, _, err := a.api.QueryRange(ctx, promql, r)
	if err != nil {
		return nil, fmt.Errorf("VM range query error: %w", err)
	}

	matrix, ok := val.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected Matrix, got %T", val)
	}

	series := make([]Series, 0, len(matrix))
	for _, stream := range matrix {
		labels := make(map[string]string)
		for k, v := range stream.Metric {
			labels[string(k)] = string(v)
		}
		values := make([]DataPoint, 0, len(stream.Values))
		for _, pair := range stream.Values {
			values = append(values, DataPoint{
				Timestamp: pair.Timestamp.Unix(),
				Value:     float64(pair.Value),
			})
		}
		series = append(series, Series{Labels: labels, Values: values})
	}

	return &QueryRangeResult{Series: series}, nil
}

func parseVMValue(val model.Value, resultType string) (*QueryResult, error) {
	switch resultType {
	case "scalar":
		vec, ok := val.(model.Vector)
		if !ok || len(vec) == 0 {
			return &QueryResult{Type: "instant", Scalar: &ScalarResult{Timestamp: time.Now().Unix()}}, nil
		}
		return &QueryResult{
			Type:   "instant",
			Scalar: &ScalarResult{Value: float64(vec[0].Value), Timestamp: vec[0].Timestamp.Unix()},
		}, nil
	case "table":
		vec, ok := val.(model.Vector)
		if !ok {
			return &QueryResult{Type: "instant", Table: []map[string]any{}}, nil
		}
		table := make([]map[string]any, 0, len(vec))
		for _, sample := range vec {
			row := make(map[string]any)
			for k, v := range sample.Metric {
				if string(k) == "__name__" {
					continue
				}
				row[string(k)] = string(v)
			}
			row["value"] = float64(sample.Value)
			table = append(table, row)
		}
		return &QueryResult{Type: "instant", Table: table}, nil
	default:
		vec, ok := val.(model.Vector)
		if !ok || len(vec) == 0 {
			return &QueryResult{Type: "instant", Scalar: &ScalarResult{Timestamp: time.Now().Unix()}}, nil
		}
		return &QueryResult{
			Type:   "instant",
			Scalar: &ScalarResult{Value: float64(vec[0].Value), Timestamp: vec[0].Timestamp.Unix()},
		}, nil
	}
}
