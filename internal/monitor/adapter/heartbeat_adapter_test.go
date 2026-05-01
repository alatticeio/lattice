package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupHeartbeatDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&PeerHeartbeat{})
	require.NoError(t, err)
	return db
}

func TestHeartbeatAdapter_Name(t *testing.T) {
	assert.Equal(t, "heartbeat", NewHeartbeatAdapter(nil).Name())
}

func TestHeartbeatAdapter_OnlineCount_Empty(t *testing.T) {
	db := setupHeartbeatDB(t)
	a := NewHeartbeatAdapter(db)
	result, err := a.Query(context.Background(), &QueryRequest{
		MetricType: "heartbeat_online_count",
		Namespace:  "wf-test",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Scalar)
	assert.Equal(t, float64(0), result.Scalar.Value)
}

func TestHeartbeatAdapter_OnlineCount_WithPeers(t *testing.T) {
	db := setupHeartbeatDB(t)
	now := time.Now()
	db.Create(&PeerHeartbeat{ID: "peer-1", Name: "node-1", NetworkID: "wf-test", LastSeen: now})
	db.Create(&PeerHeartbeat{ID: "peer-2", Name: "node-2", NetworkID: "wf-test", LastSeen: now.Add(-2 * time.Minute)}) // stale

	a := NewHeartbeatAdapter(db)
	result, err := a.Query(context.Background(), &QueryRequest{
		MetricType: "heartbeat_online_count",
		Namespace:  "wf-test",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Scalar)
	assert.Equal(t, float64(1), result.Scalar.Value)
}

func TestHeartbeatAdapter_TotalNodes(t *testing.T) {
	db := setupHeartbeatDB(t)
	db.Create(&PeerHeartbeat{ID: "peer-1", NetworkID: "wf-test", LastSeen: time.Now()})
	db.Create(&PeerHeartbeat{ID: "peer-2", NetworkID: "wf-test", LastSeen: time.Now()})
	db.Create(&PeerHeartbeat{ID: "peer-3", NetworkID: "other", LastSeen: time.Now()})

	a := NewHeartbeatAdapter(db)
	result, err := a.Query(context.Background(), &QueryRequest{
		MetricType: "heartbeat_total_nodes",
		Namespace:  "wf-test",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Scalar)
	assert.Equal(t, float64(2), result.Scalar.Value)
}

func TestHeartbeatAdapter_UnsupportedMetric(t *testing.T) {
	db := setupHeartbeatDB(t)
	a := NewHeartbeatAdapter(db)
	_, err := a.Query(context.Background(), &QueryRequest{
		MetricType: "heartbeat_unknown",
		Namespace:  "wf-test",
	})
	require.Error(t, err)
}

func TestHeartbeatAdapter_QueryRange(t *testing.T) {
	a := NewHeartbeatAdapter(nil)
	result, err := a.QueryRange(context.Background(), &QueryRangeRequest{})
	require.NoError(t, err)
	assert.Empty(t, result.Series)
}
