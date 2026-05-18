package gormstore_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupToolSpanDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.ToolSpan{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestToolSpan_WriteAndGet(t *testing.T) {
	db := setupToolSpanDB(t)
	repo := gormstore.NewToolSpanRepo(db)
	ctx := context.Background()

	span := &models.ToolSpan{
		TraceID:    "trace-001",
		AgentID:    "agent-a",
		ParentID:   "",
		Namespace:  "wf-test",
		Tool:       "list_peers",
		Status:     "ok",
		DurationMs: 42,
		StartedAt:  time.Now().UTC().Truncate(time.Second),
	}
	if err := repo.Write(ctx, span); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := repo.Get(ctx, "trace-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Tool != "list_peers" {
		t.Errorf("tool: got %q, want %q", got.Tool, "list_peers")
	}
}

func TestToolSpan_List(t *testing.T) {
	db := setupToolSpanDB(t)
	repo := gormstore.NewToolSpanRepo(db)
	ctx := context.Background()

	for i := range 3 {
		if err := repo.Write(ctx, &models.ToolSpan{
			TraceID:   fmt.Sprintf("t%d", i),
			AgentID:   "agent-a",
			Tool:      "list_peers",
			Status:    "ok",
			StartedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("Write span %d: %v", i, err)
		}
	}
	spans, err := repo.List(ctx, "agent-a", time.Time{}, time.Now().Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(spans) != 3 {
		t.Errorf("expected 3 spans, got %d", len(spans))
	}
}
