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

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentEnrollmentToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAgentEnrollmentToken_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := gormstore.NewAgentEnrollmentTokenRepo(db)
	ctx := context.Background()

	tok := &models.AgentEnrollmentToken{
		Token:            "abc123",
		AllowedNamespace: "ws-a",
		AllowedTools:     `["list_peers"]`,
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedBy:        "admin",
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByToken(ctx, "abc123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AllowedNamespace != "ws-a" {
		t.Errorf("namespace: want ws-a, got %s", got.AllowedNamespace)
	}
}

func TestAgentEnrollmentToken_MarkUsed(t *testing.T) {
	db := newTestDB(t)
	repo := gormstore.NewAgentEnrollmentTokenRepo(db)
	ctx := context.Background()

	tok := &models.AgentEnrollmentToken{
		Token:     "tok-used",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = repo.Create(ctx, tok)

	now := time.Now()
	if err := repo.MarkUsed(ctx, "tok-used", now); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	got, _ := repo.GetByToken(ctx, "tok-used")
	if got.UsedAt == nil {
		t.Error("expected UsedAt to be set")
	}
}

func TestAgentEnrollmentToken_DeleteExpired(t *testing.T) {
	db := newTestDB(t)
	repo := gormstore.NewAgentEnrollmentTokenRepo(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.AgentEnrollmentToken{
		Token:     "expired",
		ExpiresAt: time.Now().Add(-time.Hour), // already expired
	})
	_ = repo.Create(ctx, &models.AgentEnrollmentToken{
		Token:     "valid",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	if err := repo.DeleteExpired(ctx); err != nil {
		t.Fatalf("delete expired: %v", err)
	}

	_, err := repo.GetByToken(ctx, "expired")
	if err == nil {
		t.Error("expected expired token to be deleted")
	}
	_, err = repo.GetByToken(ctx, "valid")
	if err != nil {
		t.Errorf("valid token should still exist: %v", err)
	}
}
