package wireflow_exporter

import (
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// 定义一个简单的接口，让核心逻辑传数据进来
type TargetPeer struct {
	ID   string
	Name string
	IP   string
}

func NewTargetPeer(id string, name string, ip string) *TargetPeer {
	return &TargetPeer{
		ID:   id,
		Name: name,
		IP:   ip,
	}
}

func RunCycle(targets []TargetPeer) {
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(target TargetPeer) {
			defer wg.Done()

			pinger, err := probing.NewPinger("www.google.com")
			if err != nil {
				return
			}

			// 设置探测参数
			pinger.Count = 3
			pinger.Timeout = 2 * time.Second
			pinger.SetPrivileged(false) // 非 Root 运行模式

			err = pinger.Run()
			if err != nil {
				PeerLoss.WithLabelValues(target.ID).Set(100) // 探测失败视为 100% 丢包
				return
			}

			stats := pinger.Statistics()
			// 更新 Prometheus 指标
			PeerLatency.WithLabelValues(target.ID, target.Name, target.IP).Set(float64(stats.AvgRtt.Milliseconds()))
			PeerLoss.WithLabelValues(target.ID).Set(stats.PacketLoss)
		}(t)
	}
	wg.Wait()
}
