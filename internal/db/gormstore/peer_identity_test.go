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

func newPeerIdentityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.PeerIdentity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestPeerIdentity_CreateAndGetByID(t *testing.T) {
	db := newPeerIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	m := &models.PeerIdentity{
		NetworkID:          "net-1",
		Name:               "prod-db",
		PeerRef:            "node-1",
		GracePeriodSeconds: 300,
		Description:        "Production database",
	}
	if err := st.PeerIdentities().Create(ctx, m); err != nil {
		t.Fatalf("create: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected ID to be set")
	}

	got, err := st.PeerIdentities().GetByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "prod-db" {
		t.Errorf("name: want prod-db, got %s", got.Name)
	}
	if got.NetworkID != "net-1" {
		t.Errorf("network: want net-1, got %s", got.NetworkID)
	}
	if got.GracePeriodSeconds != 300 {
		t.Errorf("grace period: want 300, got %d", got.GracePeriodSeconds)
	}
}

func TestPeerIdentity_GetByNetworkAndName(t *testing.T) {
	db := newPeerIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	_ = st.PeerIdentities().Create(ctx, &models.PeerIdentity{
		NetworkID: "net-1", Name: "svc-a", PeerRef: "node-1",
	})
	_ = st.PeerIdentities().Create(ctx, &models.PeerIdentity{
		NetworkID: "net-2", Name: "svc-a", PeerRef: "node-2",
	})

	got, err := st.PeerIdentities().GetByNetworkAndName(ctx, "net-1", "svc-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PeerRef != "node-1" {
		t.Errorf("peer ref: want node-1, got %s", got.PeerRef)
	}

	_, err = st.PeerIdentities().GetByNetworkAndName(ctx, "net-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent identity")
	}
}

func TestPeerIdentity_ListByNetwork(t *testing.T) {
	db := newPeerIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	_ = st.PeerIdentities().Create(ctx, &models.PeerIdentity{NetworkID: "net-1", Name: "a", PeerRef: "n1"})
	_ = st.PeerIdentities().Create(ctx, &models.PeerIdentity{NetworkID: "net-1", Name: "b", PeerRef: "n2"})
	_ = st.PeerIdentities().Create(ctx, &models.PeerIdentity{NetworkID: "net-2", Name: "c", PeerRef: "n3"})

	list, err := st.PeerIdentities().ListByNetwork(ctx, "net-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("want 2, got %d", len(list))
	}
}

func TestPeerIdentity_Update(t *testing.T) {
	db := newPeerIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	m := &models.PeerIdentity{NetworkID: "net-1", Name: "svc", PeerRef: "node-1"}
	_ = st.PeerIdentities().Create(ctx, m)

	m.PeerRef = "node-2"
	m.Description = "updated"
	if err := st.PeerIdentities().Update(ctx, m); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := st.PeerIdentities().GetByID(ctx, m.ID)
	if got.PeerRef != "node-2" {
		t.Errorf("peer ref: want node-2, got %s", got.PeerRef)
	}
	if got.Description != "updated" {
		t.Errorf("description: want updated, got %s", got.Description)
	}
}

func TestPeerIdentity_Delete(t *testing.T) {
	db := newPeerIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	m := &models.PeerIdentity{NetworkID: "net-1", Name: "svc", PeerRef: "node-1"}
	_ = st.PeerIdentities().Create(ctx, m)

	if err := st.PeerIdentities().Delete(ctx, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = st.PeerIdentities().GetByID(ctx, m.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPeerIdentity_GracePeriod(t *testing.T) {
	db := newPeerIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	expiresAt := time.Now().Add(5 * time.Minute)
	m := &models.PeerIdentity{
		NetworkID:            "net-1",
		Name:                 "svc",
		PeerRef:              "node-new",
		PreviousPeerRef:      "node-old",
		GracePeriodSeconds:   300,
		ResolvedPeerIP:       "10.0.0.2",
		PreviousPeerIP:       "10.0.0.1",
		GracePeriodExpiresAt: &expiresAt,
	}
	_ = st.PeerIdentities().Create(ctx, m)

	got, _ := st.PeerIdentities().GetByID(ctx, m.ID)
	if got.GracePeriodExpiresAt == nil {
		t.Fatal("expected grace period expires at to be set")
	}
	if got.PreviousPeerIP != "10.0.0.1" {
		t.Errorf("previous IP: want 10.0.0.1, got %s", got.PreviousPeerIP)
	}
}
