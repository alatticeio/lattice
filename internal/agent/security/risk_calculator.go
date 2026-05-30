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
	"sync"
	"time"
)

// RiskCalculator 计算工具调用的风险评分
type RiskCalculator struct {
	frequencyMap map[string][]time.Time
	historyMap   map[string]int
	mu           sync.Mutex
	windowSize   time.Duration
	maxFrequency int
}

// NewRiskCalculator 创建一个新的风险计算器
func NewRiskCalculator() *RiskCalculator {
	return &RiskCalculator{
		frequencyMap: make(map[string][]time.Time),
		historyMap:   make(map[string]int),
		windowSize:   1 * time.Minute,
		maxFrequency: 100,
	}
}

// Calculate 计算风险评分
func (rc *RiskCalculator) Calculate(agentID string, tool string, params map[string]any) RiskScore {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	score := RiskScore{}

	// 1. 工具风险
	score.ToolRisk = GetToolRisk(tool)

	// 2. 参数风险
	checker := NewChecker()
	if err := checker.Check(tool, params); err != nil {
		score.ParamRisk = 90
	}

	// 3. 调用频率
	score.Frequency = rc.calculateFrequency(agentID)

	// 4. 历史行为
	score.History = min(rc.historyMap[agentID]*10, 100)

	return score
}

// RecordViolation 记录违规行为
func (rc *RiskCalculator) RecordViolation(agentID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.historyMap[agentID]++
}

// calculateFrequency 计算调用频率风险
func (rc *RiskCalculator) calculateFrequency(agentID string) int {
	now := time.Now()

	// 清理过期记录
	if times, ok := rc.frequencyMap[agentID]; ok {
		var valid []time.Time
		for _, t := range times {
			if now.Sub(t) <= rc.windowSize {
				valid = append(valid, t)
			}
		}
		rc.frequencyMap[agentID] = valid
	}

	// 添加当前调用
	rc.frequencyMap[agentID] = append(rc.frequencyMap[agentID], now)

	// 计算频率风险
	count := len(rc.frequencyMap[agentID])
	if count > rc.maxFrequency {
		return 100
	}

	return (count * 100) / rc.maxFrequency
}

// Reset 重置指定 agent 的统计
func (rc *RiskCalculator) Reset(agentID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	delete(rc.frequencyMap, agentID)
	delete(rc.historyMap, agentID)
}
