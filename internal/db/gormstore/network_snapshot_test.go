// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gormstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/glebarez/sqlite"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

func setupSnapshotTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.NetworkSnapshot{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNetworkSnapshotRepo_CreateAndGetByID(t *testing.T) {
	g := NewWithT(t)
	db := setupSnapshotTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	now := time.Now().UTC()
	snap := &models.NetworkSnapshot{
		ID:          "snap-001",
		WorkspaceID: "ws-test",
		Namespace:   "test-ns",
		CapturedAt:  now,
		TriggerType: "policy_change",
		TriggerBy:   "system",
		Peers:       `[{"name":"peer-a","ip":"10.100.1.1"}]`,
		Policies:    `[{"name":"allow-all","action":"ALLOW"}]`,
		Networks:    `[{"name":"default","phase":"Ready"}]`,
		Presence:    `{}`,
	}
	err = store.NetworkSnapshots().Create(ctx, snap)
	g.Expect(err).ToNot(HaveOccurred())

	got, err := store.NetworkSnapshots().GetByID(ctx, "snap-001")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got.WorkspaceID).To(Equal("ws-test"))
	g.Expect(got.TriggerType).To(Equal("policy_change"))
	g.Expect(got.Peers).To(ContainSubstring("peer-a"))
}

func TestNetworkSnapshotRepo_List_TimeRange(t *testing.T) {
	g := NewWithT(t)
	db := setupSnapshotTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	base := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	snap0 := &models.NetworkSnapshot{
		ID: "snap-0", WorkspaceID: "ws-test", CapturedAt: base, // noon
		TriggerType: "policy_change", Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	}
	snap1 := &models.NetworkSnapshot{
		ID: "snap-1", WorkspaceID: "ws-test", CapturedAt: base.Add(-1 * time.Hour), // 11am
		TriggerType: "policy_change", Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	}
	snap2 := &models.NetworkSnapshot{
		ID: "snap-2", WorkspaceID: "ws-test", CapturedAt: base.Add(-3 * time.Hour), // 9am
		TriggerType: "policy_change", Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	}
	g.Expect(store.NetworkSnapshots().Create(ctx, snap0)).To(Succeed())
	g.Expect(store.NetworkSnapshots().Create(ctx, snap1)).To(Succeed())
	g.Expect(store.NetworkSnapshots().Create(ctx, snap2)).To(Succeed())

	// Query 10am–1pm — should return snap-0 and snap-1 only
	snaps, err := store.NetworkSnapshots().List(ctx, "ws-test",
		base.Add(-2*time.Hour), base.Add(time.Hour), "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(snaps).To(HaveLen(2))
}

func TestNetworkSnapshotRepo_List_TriggerTypeFilter(t *testing.T) {
	g := NewWithT(t)
	db := setupSnapshotTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	now := time.Now().UTC()
	snap1 := &models.NetworkSnapshot{
		ID: "snap-policy", WorkspaceID: "ws-test", CapturedAt: now,
		TriggerType: "policy_change", Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	}
	snap2 := &models.NetworkSnapshot{
		ID: "snap-peer", WorkspaceID: "ws-test", CapturedAt: now,
		TriggerType: "peer_online", Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	}
	g.Expect(store.NetworkSnapshots().Create(ctx, snap1)).To(Succeed())
	g.Expect(store.NetworkSnapshots().Create(ctx, snap2)).To(Succeed())

	// Filter by policy_change
	snaps, err := store.NetworkSnapshots().List(ctx, "ws-test", now.Add(-time.Hour), now.Add(time.Hour), "policy_change")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(snaps).To(HaveLen(1))
	g.Expect(snaps[0].ID).To(Equal("snap-policy"))
}

func TestNetworkSnapshotRepo_DeleteOlderThan(t *testing.T) {
	g := NewWithT(t)
	db := setupSnapshotTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	now := time.Now().UTC()

	// Old snapshot (3 days ago)
	g.Expect(store.NetworkSnapshots().Create(ctx, &models.NetworkSnapshot{
		ID: "snap-old", WorkspaceID: "ws-test", CapturedAt: now.Add(-72 * time.Hour),
		Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	})).To(Succeed())

	// Recent snapshot
	g.Expect(store.NetworkSnapshots().Create(ctx, &models.NetworkSnapshot{
		ID: "snap-recent", WorkspaceID: "ws-test", CapturedAt: now.Add(-time.Hour),
		Peers: `[]`, Policies: `[]`, Networks: `[]`, Presence: `{}`,
	})).To(Succeed())

	n, err := store.NetworkSnapshots().DeleteOlderThan(ctx, now.Add(-48*time.Hour))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(n).To(Equal(int64(1)))

	_, err = store.NetworkSnapshots().GetByID(ctx, "snap-old")
	g.Expect(err).To(HaveOccurred())

	_, err = store.NetworkSnapshots().GetByID(ctx, "snap-recent")
	g.Expect(err).ToNot(HaveOccurred())
}

func TestNetworkSnapshotRepo_List_EmptyResult(t *testing.T) {
	g := NewWithT(t)
	db := setupSnapshotTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	now := time.Now().UTC()
	snaps, err := store.NetworkSnapshots().List(ctx, "ws-nonexistent", now.Add(-time.Hour), now, "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(snaps).To(BeEmpty())
}
