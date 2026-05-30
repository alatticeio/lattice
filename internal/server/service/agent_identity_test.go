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

func newAgentIdentitySvcDB(t *testing.T) (*gorm.DB, *gormstore.GormStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentIdentity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return db, st.(*gormstore.GormStore)
}

func TestAgentIdentityService_Create(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	result, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
		Name:            "code-review-agent",
		PeerRef:         "node-01",
		Sandbox:         "eBPF",
		AuditLevel:      "write",
		EnforcementMode: "enforce",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Name != "code-review-agent" {
		t.Errorf("name: want code-review-agent, got %s", result.Name)
	}
	if result.Sandbox != "eBPF" {
		t.Errorf("sandbox: want eBPF, got %s", result.Sandbox)
	}
	if result.Phase != "Pending" {
		t.Errorf("phase: want Pending, got %s", result.Phase)
	}
	if result.TenantID != "t1" {
		t.Errorf("tenant: want t1, got %s", result.TenantID)
	}
}

func TestAgentIdentityService_CreateWithDefaults(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	result, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
		Name:    "minimal-agent",
		PeerRef: "node-01",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Sandbox != "none" {
		t.Errorf("default sandbox: want none, got %s", result.Sandbox)
	}
	if result.AuditLevel != "write" {
		t.Errorf("default audit_level: want write, got %s", result.AuditLevel)
	}
	if result.EnforcementMode != "enforce" {
		t.Errorf("default enforcement_mode: want enforce, got %s", result.EnforcementMode)
	}
}

func TestAgentIdentityService_CreateWithArrays(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	result, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
		Name:              "tool-agent",
		PeerRef:           "node-01",
		AllowedTools:      []string{"read_file", "web_search"},
		AllowedNamespaces: []string{"default", "kube-system"},
		SpawnableRoles:    []string{"viewer", "editor"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.AllowedTools != `["read_file","web_search"]` {
		t.Errorf("tools: got %s", result.AllowedTools)
	}
	if result.AllowedNamespaces != `["default","kube-system"]` {
		t.Errorf("namespaces: got %s", result.AllowedNamespaces)
	}
	if result.SpawnableRoles != `["viewer","editor"]` {
		t.Errorf("roles: got %s", result.SpawnableRoles)
	}
}

func TestAgentIdentityService_List(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	for _, name := range []string{"agent-a", "agent-b"} {
		_, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
			Name:    name,
			PeerRef: "node-01",
		})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	list, err := svc.List(ctx, "t1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("want 2, got %d", len(list))
	}
}

func TestAgentIdentityService_Get(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	created, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
		Name:    "get-agent",
		PeerRef: "node-01",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "get-agent" {
		t.Errorf("name: want get-agent, got %s", got.Name)
	}
}

func TestAgentIdentityService_Update(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	created, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
		Name:    "update-agent",
		PeerRef: "node-01",
		Sandbox: "none",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := svc.Update(ctx, created.ID, service.CreateAgentIdentityRequest{
		Name:    "update-agent",
		PeerRef: "node-02",
		Sandbox: "gVisor",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.PeerRef != "node-02" {
		t.Errorf("peer ref: want node-02, got %s", updated.PeerRef)
	}
	if updated.Sandbox != "gVisor" {
		t.Errorf("sandbox: want gVisor, got %s", updated.Sandbox)
	}
}

func TestAgentIdentityService_Delete(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	created, err := svc.Create(ctx, "t1", service.CreateAgentIdentityRequest{
		Name:    "delete-agent",
		PeerRef: "node-01",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = svc.Get(ctx, created.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAgentIdentityService_NotFound(t *testing.T) {
	_, st := newAgentIdentitySvcDB(t)
	svc := service.NewAgentIdentityService(st)
	ctx := context.Background()

	_, err := svc.Get(ctx, "non-existent-id")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}
