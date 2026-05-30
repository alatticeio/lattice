// Copyright 2026 The Lattice Authors, Inc.
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

package security

import (
	"testing"
)

func TestRiskScore(t *testing.T) {
	tests := []struct {
		name      string
		score     RiskScore
		total     int
		average   int
		riskLevel string
	}{
		{
			name:      "low risk",
			score:     RiskScore{ToolRisk: 20, ParamRisk: 0, Frequency: 0, History: 0},
			total:     20,
			average:   5,
			riskLevel: "none",
		},
		{
			name:      "medium risk",
			score:     RiskScore{ToolRisk: 40, ParamRisk: 30, Frequency: 20, History: 10},
			total:     100,
			average:   25,
			riskLevel: "low",
		},
		{
			name:      "high risk",
			score:     RiskScore{ToolRisk: 80, ParamRisk: 90, Frequency: 60, History: 50},
			total:     280,
			average:   70,
			riskLevel: "high",
		},
		{
			name:      "critical risk",
			score:     RiskScore{ToolRisk: 100, ParamRisk: 100, Frequency: 100, History: 100},
			total:     400,
			average:   100,
			riskLevel: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.score.Total(); got != tt.total {
				t.Errorf("Total() = %v, want %v", got, tt.total)
			}
			if got := tt.score.Average(); got != tt.average {
				t.Errorf("Average() = %v, want %v", got, tt.average)
			}
			if got := tt.score.RiskLevel(); got != tt.riskLevel {
				t.Errorf("RiskLevel() = %v, want %v", got, tt.riskLevel)
			}
		})
	}
}

func TestGetToolRisk(t *testing.T) {
	tests := []struct {
		tool string
		risk int
	}{
		{"shell:exec", 80},
		{"exec:run", 80},
		{"file:write", 60},
		{"file:delete", 60},
		{"file:read", 30},
		{"db:query", 40},
		{"sql:execute", 40},
		{"http:request", 50},
		{"api:call", 50},
		{"db:write", 70},
		{"db:delete", 70},
		{"unknown:tool", 20},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			if got := GetToolRisk(tt.tool); got != tt.risk {
				t.Errorf("GetToolRisk(%v) = %v, want %v", tt.tool, got, tt.risk)
			}
		})
	}
}

func TestRiskCalculator(t *testing.T) {
	calculator := NewRiskCalculator()

	t.Run("safe tool call", func(t *testing.T) {
		score := calculator.Calculate("agent-1", "file:read", map[string]any{
			"path": "data/config.json",
		})

		if score.ToolRisk != 30 {
			t.Errorf("ToolRisk = %v, want 30", score.ToolRisk)
		}
		if score.ParamRisk != 0 {
			t.Errorf("ParamRisk = %v, want 0", score.ParamRisk)
		}
	})

	t.Run("dangerous tool call", func(t *testing.T) {
		score := calculator.Calculate("agent-1", "shell:exec", map[string]any{
			"command": "rm -rf /",
		})

		if score.ToolRisk != 80 {
			t.Errorf("ToolRisk = %v, want 80", score.ToolRisk)
		}
		if score.ParamRisk != 90 {
			t.Errorf("ParamRisk = %v, want 90", score.ParamRisk)
		}
	})

	t.Run("record violation", func(t *testing.T) {
		calculator.RecordViolation("agent-2")
		calculator.RecordViolation("agent-2")

		score := calculator.Calculate("agent-2", "file:read", nil)
		if score.History != 20 {
			t.Errorf("History = %v, want 20", score.History)
		}
	})

	t.Run("reset", func(t *testing.T) {
		calculator.Reset("agent-2")

		score := calculator.Calculate("agent-2", "file:read", nil)
		if score.History != 0 {
			t.Errorf("History = %v, want 0", score.History)
		}
	})
}
