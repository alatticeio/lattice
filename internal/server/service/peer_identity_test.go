package service_test

import (
	"context"
	"testing"

	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPeerIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PeerIdentity{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newPeerIdentityTestStore(t *testing.T) *gormstore.GormStore {
	t.Helper()
	db := newPeerIdentityTestDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	return st.(*gormstore.GormStore)
}

func TestPeerIdentityService_Create(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	got, err := svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{
		Name:    "prod-db",
		PeerRef: "node-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected ID")
	}
	if got.Name != "prod-db" {
		t.Errorf("name: want prod-db, got %s", got.Name)
	}
	if got.GracePeriodSeconds != 300 {
		t.Errorf("default grace period: want 300, got %d", got.GracePeriodSeconds)
	}
}

func TestPeerIdentityService_CreateWithGracePeriod(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	got, err := svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{
		Name:               "svc",
		PeerRef:            "node-1",
		GracePeriodSeconds: 600,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.GracePeriodSeconds != 600 {
		t.Errorf("grace period: want 600, got %d", got.GracePeriodSeconds)
	}
}

func TestPeerIdentityService_List(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	_, _ = svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{Name: "a", PeerRef: "n1"})
	_, _ = svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{Name: "b", PeerRef: "n2"})
	_, _ = svc.Create(ctx, "net-2", service.CreatePeerIdentityRequest{Name: "c", PeerRef: "n3"})

	list, err := svc.List(ctx, "net-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("want 2, got %d", len(list))
	}
}

func TestPeerIdentityService_Get(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{Name: "svc", PeerRef: "node-1"})

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "svc" {
		t.Errorf("name: want svc, got %s", got.Name)
	}
}

func TestPeerIdentityService_GetByNetworkAndName(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	_, _ = svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{Name: "svc", PeerRef: "node-1"})

	got, err := svc.GetByNetworkAndName(ctx, "net-1", "svc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PeerRef != "node-1" {
		t.Errorf("peer ref: want node-1, got %s", got.PeerRef)
	}
}

func TestPeerIdentityService_Update(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{Name: "svc", PeerRef: "node-1"})

	updated, err := svc.Update(ctx, created.ID, service.CreatePeerIdentityRequest{
		Name:    "svc",
		PeerRef: "node-2",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.PeerRef != "node-2" {
		t.Errorf("peer ref: want node-2, got %s", updated.PeerRef)
	}
}

func TestPeerIdentityService_Delete(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "net-1", service.CreatePeerIdentityRequest{Name: "svc", PeerRef: "node-1"})

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.Get(ctx, created.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPeerIdentityService_NotFound(t *testing.T) {
	st := newPeerIdentityTestStore(t)
	svc := service.NewPeerIdentityService(st)
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}
