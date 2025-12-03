// Copyright 2025 wireflowio.com, Inc.
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

package collector

import "time"

type MetricCollector interface {
	Collect() ([]Metric, error)
	Name() string
}

type Metric interface {
	Name() string
	Value() interface{}
	Labels() map[string]string
	Timestamp() time.Time
	Help() string
}

type SimpleMetric struct {
	name      string
	value     interface{}
	labels    map[string]string
	timestamp time.Time
	help      string
}

func NewSimpleMetric(name string, value interface{}, labels map[string]string, timestamp time.Time, help string) *SimpleMetric {
	return &SimpleMetric{
		name:      name,
		value:     value,
		labels:    labels,
		timestamp: timestamp,
		help:      help,
	}
}

func (m *SimpleMetric) Name() string {
	return m.name
}

func (m *SimpleMetric) Value() interface{} {
	return m.value
}

func (m *SimpleMetric) Labels() map[string]string {
	return m.labels
}

func (m *SimpleMetric) Timestamp() time.Time {
	return m.timestamp
}

func (m *SimpleMetric) Help() string {
	return m.help
}

type Query struct{}

type Alert struct{}

type Storage interface {
	Store(metrics []Metric) error
	Query(query Query) ([]Metric, error)
}

type Alerter interface {
	Evaluate(metrics []Metric) ([]Alert, error)
	Send(alerts []Alert) error
}
