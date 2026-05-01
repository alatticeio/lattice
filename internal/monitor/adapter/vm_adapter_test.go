package adapter

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVMValue_Scalar(t *testing.T) {
	vec := model.Vector{
		&model.Sample{
			Metric:    model.Metric{"peer_id": "test-peer"},
			Value:     model.SampleValue(42.5),
			Timestamp: model.Time(1705312800000),
		},
	}
	result, err := parseVMValue(vec, "scalar")
	require.NoError(t, err)
	require.NotNil(t, result.Scalar)
	assert.Equal(t, 42.5, result.Scalar.Value)
}

func TestParseVMValue_EmptyVector(t *testing.T) {
	vec := model.Vector{}
	result, err := parseVMValue(vec, "scalar")
	require.NoError(t, err)
	require.NotNil(t, result.Scalar)
	assert.Equal(t, float64(0), result.Scalar.Value)
}

func TestParseVMValue_Table(t *testing.T) {
	vec := model.Vector{
		&model.Sample{
			Metric:    model.Metric{"peer_id": "abc", "__name__": "lattice_cpu"},
			Value:     model.SampleValue(72.4),
			Timestamp: model.Time(1705312800000),
		},
	}
	result, err := parseVMValue(vec, "table")
	require.NoError(t, err)
	require.Len(t, result.Table, 1)
	assert.Equal(t, "abc", result.Table[0]["peer_id"])
	assert.Equal(t, 72.4, result.Table[0]["value"])
	assert.Nil(t, result.Table[0]["__name__"])
}

func TestParseVMValue_DefaultFallback(t *testing.T) {
	vec := model.Vector{
		&model.Sample{
			Value:     model.SampleValue(99.0),
			Timestamp: model.Time(1705312800000),
		},
	}
	result, err := parseVMValue(vec, "unknown_type")
	require.NoError(t, err)
	require.NotNil(t, result.Scalar)
	assert.Equal(t, 99.0, result.Scalar.Value)
}
