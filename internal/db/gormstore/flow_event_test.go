package gormstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupFlowEventDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&models.FlowEvent{})
	return db
}

func TestFlowEvent_WriteAndListByTrace(t *testing.T) {
	db := setupFlowEventDB(t)
	repo := gormstore.NewFlowEventRepo(db)
	ctx := context.Background()

	_ = repo.Write(ctx, &models.FlowEvent{
		TraceID: "trace-001", AgentID: "agent-a",
		Direction: "egress", DstIP: "10.0.0.1", DstPort: 443, Bytes: 1024,
		Ts: time.Now().UTC(),
	})

	events, err := repo.ListByTrace(ctx, "trace-001")
	if err != nil {
		t.Fatalf("ListByTrace: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].DstPort != 443 {
		t.Errorf("port: got %d, want 443", events[0].DstPort)
	}
}
