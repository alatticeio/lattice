package collector

import (
	"fmt"
	"time"
	"wireflow/internal/infra"

	"golang.zx2c4.com/wireguard/wgctrl"
)

type PeerStatusCollector struct {
	peerManager *infra.PeerManager
}

func NewPeerStatusCollector() *PeerStatusCollector {
	return &PeerStatusCollector{
		peerManager: infra.NewPeerManager(),
	}
}

func (c *PeerStatusCollector) Name() string {
	return "peer_status"
}

func (c *PeerStatusCollector) Collect() ([]Metric, error) {
	metrics := make([]Metric, 0)
	now := time.Now()

	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to init wgctrl: %v", err)
	}
	defer client.Close()

	// 1. 获取 WireGuard 设备信息
	device, err := client.Device("wg0") // 建议 wg0 做成可配置的
	if err != nil {
		return nil, err
	}

	for _, peer := range device.Peers {

		// 2. 从 peerManager 获取业务层面的元数据 (IP/Alias)
		// 假设你的 peerManager 存储了公钥到信息的映射
		alias := "unknown"
		internalIP := "0.0.0.0"
		peerId := infra.FromKey(peer.PublicKey).ToUint64()
		cachedPeer := c.peerManager.GetPeer(peerId)
		if cachedPeer != nil {
			alias = cachedPeer.Name
			internalIP = *cachedPeer.Address
		}

		// 3. 状态计算：3 分钟内有握手视为在线
		status := 0.0
		if !peer.LastHandshakeTime.IsZero() && now.Sub(peer.LastHandshakeTime) < 3*time.Minute {
			status = 1.0
		}

		// 4. 封装为你的通用 Metric 结构
		// 增加更多维度的标签，方便在 Grafana 里进行筛选
		labels := map[string]string{
			"peer_id": string(peerId), // 取公钥前8位作为 ID
			"alias":   alias,          // 节点别名
			"ip":      internalIP,     // 隧道内 IP
		}

		metrics = append(metrics, NewSimpleMetric(
			"peer_status",
			status,
			labels,
			now,
			"WireGuard peer connection status",
		))

		// 额外红利：顺便把流量也统计了
		metrics = append(metrics, NewSimpleMetric(
			"peer_receive_bytes",
			float64(peer.ReceiveBytes),
			labels,
			now,
			"Total bytes received from peer",
		))
		metrics = append(metrics, NewSimpleMetric(
			"peer_transmit_bytes",
			float64(peer.TransmitBytes),
			labels,
			now,
			"Total bytes transmitted to peer",
		))
	}

	return metrics, nil
}
