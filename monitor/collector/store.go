// Copyright 2025 The Wireflow Authors, Inc.
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

import (
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type PrometheusStorage struct {
	database string
	pusher   *push.Pusher
}

func NewPrometheusStorage(database string) *PrometheusStorage {
	return &PrometheusStorage{
		database: "http://pushgateway.linkany.io:9091",
		pusher:   push.New(database, "prometheus"),
	}
}

// Store push data to pushgateway, prometheus will pull data from the pushgateway.
func (s *PrometheusStorage) Store(metrics []Metric) error {
	if err := push.New("http://pushgateway.linkany.io:9091", "db-backup").Collector(s.process(metrics[0])).Collector(s.process(metrics[1])).Grouping("db", "linkany").Push(); err != nil {
		fmt.Println("Could not push completion time to pushgateway:", err)
	}

	return nil
}

func (s *PrometheusStorage) process(metric Metric) prometheus.Gauge {
	data := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: metric.Name(),
		Help: metric.Help(),
	})

	if _, b := metric.Value().(float64); b {
		data.Set(metric.Value().(float64))
	} else {
		value, _ := strconv.ParseFloat(fmt.Sprintf("%d", metric.Value().(int64)), 64)
		data.Set(value)
	}
	data.SetToCurrentTime()
	return data
}

func (s *PrometheusStorage) Query(query Query) ([]Metric, error) {
	return nil, nil
}
