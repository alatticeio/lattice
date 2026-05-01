package alert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCompareThreshold(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		operator  string
		threshold float64
		expected  bool
	}{
		{"gt true", 10, "gt", 5, true},
		{"gt false (equal)", 10, "gt", 10, false},
		{"gte true (equal)", 10, "gte", 10, true},
		{"lt true", 10, "lt", 15, true},
		{"lt false", 10, "lt", 5, false},
		{"lte true (equal)", 10, "lte", 10, true},
		{"eq true", 10, "eq", 10, true},
		{"eq false", 10, "eq", 5, false},
		{"neq true", 10, "neq", 5, true},
		{"neq false", 10, "neq", 10, false},
		{"invalid operator", 10, "invalid", 5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, compareThreshold(tc.value, tc.operator, tc.threshold))
		})
	}
}

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil, nil, nil)
	assert.NotNil(t, engine)
	assert.Equal(t, 30*time.Second, engine.evalInterval)
	assert.NotNil(t, engine.activeAlerts)
	assert.Empty(t, engine.activeAlerts)
}
