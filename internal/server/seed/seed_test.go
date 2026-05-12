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

package seed_test

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/seed"
	"github.com/glebarez/sqlite"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Injector", func() {
	var (
		st  store.Store
		ctx context.Context
	)

	BeforeEach(func() {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())
		st, err = gormstore.New(db)
		Expect(err).NotTo(HaveOccurred())
		ctx = context.Background()
	})

	It("injects audit logs tagged with IsSeed=true", func() {
		injector := seed.NewInjector(st, nil)
		err := injector.Inject(ctx, "ws-test-001", seed.Options{
			VirtualNodes: 2,
			HistoryDays:  3,
			AuditEntries: 5,
		})
		Expect(err).NotTo(HaveOccurred())

		logs, total, err := st.AuditLogs().List(ctx, store.AuditLogFilter{
			WorkspaceID: "ws-test-001",
			PageSize:    20,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(total).To(BeNumerically(">=", int64(5)))
		Expect(logs[0].IsSeed).To(BeTrue())
	})

	It("injects 3 policies tagged with IsSeed=true", func() {
		injector := seed.NewInjector(st, nil)
		err := injector.Inject(ctx, "ws-test-002", seed.DefaultOptions())
		Expect(err).NotTo(HaveOccurred())

		policies, _, err := st.Policies().List(ctx, store.PolicyFilter{
			WorkspaceID: "ws-test-002",
			PageSize:    20,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(policies)).To(BeNumerically(">=", 3))
		Expect(policies[0].IsSeed).To(BeTrue())
	})

	It("Clear removes all seed records", func() {
		injector := seed.NewInjector(st, nil)
		Expect(injector.Inject(ctx, "ws-test-003", seed.DefaultOptions())).To(Succeed())
		Expect(injector.Clear(ctx, "ws-test-003")).To(Succeed())

		_, auditTotal, err := st.AuditLogs().List(ctx, store.AuditLogFilter{
			WorkspaceID: "ws-test-003",
			PageSize:    20,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(auditTotal).To(Equal(int64(0)))

		policies, _, err := st.Policies().List(ctx, store.PolicyFilter{
			WorkspaceID: "ws-test-003",
			PageSize:    20,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(policies)).To(Equal(0))

		_, alertTotal, err := st.Alerts().ListAlertHistory(ctx, "ws-test-003", 1, 20)
		Expect(err).NotTo(HaveOccurred())
		Expect(alertTotal).To(Equal(int64(0)))
	})
})
