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

func newAgentIdentityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentIdentity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAgentIdentity_CreateAndGetByID(t *testing.T) {
	db := newAgentIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	m := &models.AgentIdentity{
		TenantID:        "t1",
		Name:            "code-review-agent",
		PeerRef:         "node-01",
		AllowedTools:    `["read_file","list_dir"]`,
		Sandbox:         "eBPF",
		AuditLevel:      "write",
		EnforcementMode: "enforce",
		Phase:           "Active",
	}
	if err := st.AgentIdentities().Create(ctx, m); err != nil {
		t.Fatalf("create: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected ID to be set")
	}

	got, err := st.AgentIdentities().GetByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "code-review-agent" {
		t.Errorf("name: want code-review-agent, got %s", got.Name)
	}
	if got.Sandbox != "eBPF" {
		t.Errorf("sandbox: want eBPF, got %s", got.Sandbox)
	}
	if got.AllowedTools != `["read_file","list_dir"]` {
		t.Errorf("allowed tools: want [\"read_file\",\"list_dir\"], got %s", got.AllowedTools)
	}
}

func TestAgentIdentity_GetByTenantAndName(t *testing.T) {
	db := newAgentIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	_ = st.AgentIdentities().Create(ctx, &models.AgentIdentity{
		TenantID: "t1", Name: "agent-a", PeerRef: "n1",
	})
	_ = st.AgentIdentities().Create(ctx, &models.AgentIdentity{
		TenantID: "t2", Name: "agent-a", PeerRef: "n2",
	})

	got, err := st.AgentIdentities().GetByTenantAndName(ctx, "t1", "agent-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PeerRef != "n1" {
		t.Errorf("peer ref: want n1, got %s", got.PeerRef)
	}

	_, err = st.AgentIdentities().GetByTenantAndName(ctx, "t1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent identity")
	}
}

func TestAgentIdentity_ListByTenant(t *testing.T) {
	db := newAgentIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	_ = st.AgentIdentities().Create(ctx, &models.AgentIdentity{TenantID: "t1", Name: "a", PeerRef: "n1"})
	_ = st.AgentIdentities().Create(ctx, &models.AgentIdentity{TenantID: "t1", Name: "b", PeerRef: "n2"})
	_ = st.AgentIdentities().Create(ctx, &models.AgentIdentity{TenantID: "t2", Name: "c", PeerRef: "n3"})

	list, err := st.AgentIdentities().ListByTenant(ctx, "t1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("want 2, got %d", len(list))
	}
}

func TestAgentIdentity_Update(t *testing.T) {
	db := newAgentIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	m := &models.AgentIdentity{TenantID: "t1", Name: "agent", PeerRef: "node-01", Sandbox: "none"}
	_ = st.AgentIdentities().Create(ctx, m)

	m.PeerRef = "node-02"
	m.Sandbox = "gVisor"
	m.AllowedTools = `["web_search"]`
	if err := st.AgentIdentities().Update(ctx, m); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := st.AgentIdentities().GetByID(ctx, m.ID)
	if got.PeerRef != "node-02" {
		t.Errorf("peer ref: want node-02, got %s", got.PeerRef)
	}
	if got.Sandbox != "gVisor" {
		t.Errorf("sandbox: want gVisor, got %s", got.Sandbox)
	}
}

func TestAgentIdentity_Delete(t *testing.T) {
	db := newAgentIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	m := &models.AgentIdentity{TenantID: "t1", Name: "agent", PeerRef: "node-01"}
	_ = st.AgentIdentities().Create(ctx, m)

	if err := st.AgentIdentities().Delete(ctx, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = st.AgentIdentities().GetByID(ctx, m.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAgentIdentity_Expiry(t *testing.T) {
	db := newAgentIdentityDB(t)
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	expiry := time.Now().Add(24 * time.Hour)
	m := &models.AgentIdentity{
		TenantID:  "t1",
		Name:      "temp-agent",
		PeerRef:   "node-01",
		ExpiresAt: &expiry,
	}
	_ = st.AgentIdentities().Create(ctx, m)

	got, _ := st.AgentIdentities().GetByID(ctx, m.ID)
	if got.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}
	if got.ExpiresAt.Unix() != expiry.Unix() {
		t.Errorf("expiry: want %v, got %v", expiry, got.ExpiresAt)
	}
}
