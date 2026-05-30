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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
)

var _ = Describe("IdentityResolver", func() {
	var (
		resolver *IdentityResolver
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("without client", func() {
		It("returns error when client not initialized", func() {
			resolver = NewIdentityResolver()
			_, err := resolver.ResolveIdentity(ctx, "net-1", "test-identity")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("client not initialized"))
		})
	})

	Describe("with fake client", func() {
		var fakeClient client.Client

		BeforeEach(func() {
			fakeClient = k8sClient
			resolver = NewIdentityResolverWithClient(fakeClient)
		})

		Context("when PeerIdentity exists and is bound", func() {
			const identityName = "test-resolver-identity"

			BeforeEach(func() {
				pi := &v1alpha1.PeerIdentity{
					ObjectMeta: metav1.ObjectMeta{Name: identityName},
					Spec: v1alpha1.PeerIdentitySpec{
						Network: "net-1",
						PeerRef: "test-peer",
					},
					Status: v1alpha1.PeerIdentityStatus{
						ResolvedPeerIP: "10.0.0.5",
					},
				}
				Expect(fakerCreate(ctx, pi)).To(Succeed())
			})

			AfterEach(func() {
				pi := &v1alpha1.PeerIdentity{}
				if err := fakeClient.Get(ctx, types.NamespacedName{Name: identityName}, pi); err == nil {
					_ = fakeClient.Delete(ctx, pi)
				}
			})

			It("resolves to the peer IP", func() {
				ips, err := resolver.ResolveIdentity(ctx, "net-1", identityName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ContainElement("10.0.0.5"))
			})

			It("returns empty for wrong network", func() {
				ips, err := resolver.ResolveIdentity(ctx, "net-2", identityName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(BeEmpty())
			})
		})

		Context("when PeerIdentity has grace period active", func() {
			const identityName = "test-grace-identity"

			BeforeEach(func() {
				expiresAt := metav1.Now().Add(5 * 60 * 1e9) // 5 min from now
				pi := &v1alpha1.PeerIdentity{
					ObjectMeta: metav1.ObjectMeta{Name: identityName},
					Spec: v1alpha1.PeerIdentitySpec{
						Network:         "net-1",
						PeerRef:         "new-peer",
						PreviousPeerRef: "old-peer",
					},
					Status: v1alpha1.PeerIdentityStatus{
						ResolvedPeerIP:       "10.0.0.10",
						PreviousPeerIP:       "10.0.0.9",
						GracePeriodExpiresAt: &metav1.Time{Time: expiresAt},
					},
				}
				Expect(fakerCreate(ctx, pi)).To(Succeed())
			})

			AfterEach(func() {
				pi := &v1alpha1.PeerIdentity{}
				if err := fakeClient.Get(ctx, types.NamespacedName{Name: identityName}, pi); err == nil {
					_ = fakeClient.Delete(ctx, pi)
				}
			})

			It("returns both current and previous IPs", func() {
				ips, err := resolver.ResolveIdentity(ctx, "net-1", identityName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("10.0.0.10"))
				Expect(ips).To(ContainElement("10.0.0.9"))
			})
		})

		Context("when PeerIdentity does not exist", func() {
			It("returns empty list without error", func() {
				ips, err := resolver.ResolveIdentity(ctx, "net-1", "nonexistent")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(BeEmpty())
			})
		})

		Context("when identityName is empty", func() {
			It("returns nil", func() {
				ips, err := resolver.ResolveIdentity(ctx, "net-1", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(BeNil())
			})
		})

		Context("ResolveIdentities batch", func() {
			const id1 = "batch-id-1"
			const id2 = "batch-id-2"

			BeforeEach(func() {
				pi1 := &v1alpha1.PeerIdentity{
					ObjectMeta: metav1.ObjectMeta{Name: id1},
					Spec:       v1alpha1.PeerIdentitySpec{Network: "net-1", PeerRef: "p1"},
					Status:     v1alpha1.PeerIdentityStatus{ResolvedPeerIP: "10.0.0.20"},
				}
				pi2 := &v1alpha1.PeerIdentity{
					ObjectMeta: metav1.ObjectMeta{Name: id2},
					Spec:       v1alpha1.PeerIdentitySpec{Network: "net-1", PeerRef: "p2"},
					Status:     v1alpha1.PeerIdentityStatus{ResolvedPeerIP: "10.0.0.21"},
				}
				Expect(fakerCreate(ctx, pi1)).To(Succeed())
				Expect(fakerCreate(ctx, pi2)).To(Succeed())
			})

			AfterEach(func() {
				for _, name := range []string{id1, id2} {
					pi := &v1alpha1.PeerIdentity{}
					if err := fakeClient.Get(ctx, types.NamespacedName{Name: name}, pi); err == nil {
						_ = fakeClient.Delete(ctx, pi)
					}
				}
			})

			It("resolves multiple identities", func() {
				ips := resolver.ResolveIdentities(ctx, "net-1", []string{id1, id2, "nonexistent"})
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("10.0.0.20"))
				Expect(ips).To(ContainElement("10.0.0.21"))
			})
		})
	})
})

// fakerCreate creates a K8s object using the shared fake client.
func fakerCreate(ctx context.Context, obj client.Object) error {
	return k8sClient.Create(ctx, obj)
}
