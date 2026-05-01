package adapter

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PeerHeartbeat represents a peer heartbeat record in the database.
type PeerHeartbeat struct {
	ID           string    `gorm:"column:id"`
	Name         string    `gorm:"column:name"`
	NetworkID    string    `gorm:"column:network_id"`
	LastSeen     time.Time `gorm:"column:last_seen"`
	EndpointAddr string    `gorm:"column:endpoint_addr"`
}

// HeartbeatAdapter queries the database for peer heartbeat status.
type HeartbeatAdapter struct {
	db *gorm.DB
}

// NewHeartbeatAdapter creates a new HeartbeatAdapter.
func NewHeartbeatAdapter(db *gorm.DB) *HeartbeatAdapter {
	return &HeartbeatAdapter{db: db}
}

func (a *HeartbeatAdapter) Name() string { return "heartbeat" }

func (a *HeartbeatAdapter) Health(ctx context.Context) error {
	return a.db.WithContext(ctx).Raw("SELECT 1").Scan(nil).Error
}

func (a *HeartbeatAdapter) Query(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	result := &QueryResult{Type: ResultInstant}
	now := time.Now()
	threshold := now.Add(-90 * time.Second)

	switch req.MetricType {
	case "heartbeat_online_count":
		var count int64
		if err := a.db.WithContext(ctx).Model(&PeerHeartbeat{}).
			Where("network_id = ? AND last_seen > ?", req.Namespace, threshold).
			Count(&count).Error; err != nil {
			return nil, fmt.Errorf("query online count: %w", err)
		}
		result.Scalar = &ScalarResult{Value: float64(count), Timestamp: now.Unix()}

	case "heartbeat_total_nodes":
		var count int64
		if err := a.db.WithContext(ctx).Model(&PeerHeartbeat{}).
			Where("network_id = ?", req.Namespace).
			Count(&count).Error; err != nil {
			return nil, fmt.Errorf("query total nodes: %w", err)
		}
		result.Scalar = &ScalarResult{Value: float64(count), Timestamp: now.Unix()}

	case "heartbeat_peer_status":
		var peers []PeerHeartbeat
		if err := a.db.WithContext(ctx).
			Where("network_id = ?", req.Namespace).
			Find(&peers).Error; err != nil {
			return nil, fmt.Errorf("query peer status: %w", err)
		}
		table := make([]map[string]any, 0, len(peers))
		for _, p := range peers {
			table = append(table, map[string]any{
				"peer_id":   p.ID,
				"name":      p.Name,
				"online":    !p.LastSeen.Before(threshold),
				"last_seen": p.LastSeen.Unix(),
			})
		}
		result.Table = table

	default:
		return nil, fmt.Errorf("unsupported heartbeat metric: %s: %w", req.MetricType, ErrMetricNotFound)
	}

	return result, nil
}

func (a *HeartbeatAdapter) QueryRange(ctx context.Context, req *QueryRangeRequest) (*QueryRangeResult, error) {
	return &QueryRangeResult{Series: []Series{}}, nil
}
