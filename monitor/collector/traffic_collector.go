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

import (
	"fmt"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
)

type TrafficCollector struct {
}

func (t *TrafficCollector) Name() string {
	return "TrafficCollector"
}

func (t *TrafficCollector) Collect() ([]Metric, error) {
	var metrics []Metric

	// get traffic data from wireguard
	ctr, _ := wgctrl.New()
	devices, _ := ctr.Devices()
	if len(devices) > 0 {
		peers := devices[0].Peers
		var allTrafficeIn int64
		var allTrafficeOut int64
		for _, peer := range peers {
			allTrafficeIn += peer.ReceiveBytes
			allTrafficeOut += peer.TransmitBytes
			metrics = append(metrics, NewSimpleMetric(
				fmt.Sprintf("%s_%s", peer.PublicKey, "traffic_in"),
				peer.ReceiveBytes,
				map[string]string{"peer": peer.PublicKey.String()},
				time.Now(),
				"current traffic in",
			))
			metrics = append(metrics, NewSimpleMetric(
				fmt.Sprintf("%s_%s", peer.PublicKey, "traffic_out"), peer.TransmitBytes,
				map[string]string{"peer": peer.PublicKey.String()},
				time.Now(),
				"current traffic out",
			))
		}

		metrics = append(metrics, NewSimpleMetric(
			"all_traffic_in",
			allTrafficeIn,
			map[string]string{"device": devices[0].Name},
			time.Now(),
			"all traffic in",
		))

		metrics = append(metrics, NewSimpleMetric(
			"all_traffic_out",
			allTrafficeOut,
			map[string]string{"device": devices[0].Name},
			time.Now(),
			"all traffic out",
		))

	}

	return metrics, nil
}
