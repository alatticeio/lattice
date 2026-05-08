package vm

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// QuerySingleValue wraps the official API and returns a float64 directly
func QuerySingleValue(ctx context.Context, api v1.API, query string) (float64, error) {
	// Execute the query — note this returns model.Value interface
	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}

	// Type conversion: Prometheus instant queries usually return Vector
	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		// If no data found, return 0
		return 0, nil
	}

	// Take the first sample value from the Vector
	return float64(vector[0].Value), nil
}
