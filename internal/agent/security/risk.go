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

// RiskScore 表示风险评分
type RiskScore struct {
	ToolRisk  int `json:"tool_risk"`
	ParamRisk int `json:"param_risk"`
	Frequency int `json:"frequency"`
	History   int `json:"history"`
}

// Total 返回总分
func (r RiskScore) Total() int {
	return r.ToolRisk + r.ParamRisk + r.Frequency + r.History
}

// Average 返回平均分
func (r RiskScore) Average() int {
	return r.Total() / 4
}

// RiskLevel 返回风险等级
func (r RiskScore) RiskLevel() string {
	avg := r.Average()
	switch {
	case avg >= 80:
		return "critical"
	case avg >= 60:
		return "high"
	case avg >= 40:
		return "medium"
	case avg >= 20:
		return "low"
	default:
		return "none"
	}
}

// GetToolRisk 根据工具类型返回风险分数
func GetToolRisk(tool string) int {
	switch tool {
	case "shell:exec", "exec:run":
		return 80
	case "file:write", "file:delete":
		return 60
	case "file:read":
		return 30
	case "db:query", "sql:execute":
		return 40
	case "http:request", "api:call":
		return 50
	case "db:write", "db:delete":
		return 70
	default:
		return 20
	}
}
