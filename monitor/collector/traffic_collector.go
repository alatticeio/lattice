package collector

import (
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl"
	"time"
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
