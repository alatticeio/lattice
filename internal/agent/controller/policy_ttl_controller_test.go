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
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newPolicyTTLScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func newPolicyFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(newPolicyTTLScheme()).WithObjects(objs...).Build()
}

var _ = Describe("PolicyTTLReconciler", func() {
	Context("when policy has passed expiresAt", func() {
		It("deletes the expired policy", func() {
			policy := &v1alpha1.LatticePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "temp-access",
					Namespace: "default",
				},
				Spec: v1alpha1.LatticePolicySpec{
					Network:   "test-net",
					Action:    "ALLOW",
					ExpiresAt: &metav1.Time{Time: time.Now().Add(-time.Minute)},
				},
			}

			fc := newPolicyFakeClient(policy)
			r := NewPolicyTTLReconciler(fc)
			_, err := r.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: "default", Name: "temp-access"},
			})
			Expect(err).ToNot(HaveOccurred())

			var got v1alpha1.LatticePolicy
			getErr := fc.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "temp-access"}, &got)
			Expect(apierrors.IsNotFound(getErr)).To(BeTrue(), "expired policy should be deleted")
		})
	})

	Context("when policy expiresAt is in the future", func() {
		It("requeues without deleting", func() {
			policy := &v1alpha1.LatticePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "temp-access-live",
					Namespace: "default",
				},
				Spec: v1alpha1.LatticePolicySpec{
					Network:   "test-net",
					Action:    "ALLOW",
					ExpiresAt: &metav1.Time{Time: time.Now().Add(time.Hour)},
				},
			}

			fc := newPolicyFakeClient(policy)
			r := NewPolicyTTLReconciler(fc)
			result, err := r.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: "default", Name: "temp-access-live"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			var got v1alpha1.LatticePolicy
			Expect(fc.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "temp-access-live"}, &got)).To(Succeed())
		})
	})

	Context("when policy has no expiresAt", func() {
		It("does nothing", func() {
			policy := &v1alpha1.LatticePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "permanent-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.LatticePolicySpec{
					Network: "test-net",
					Action:  "DENY",
				},
			}

			fc := newPolicyFakeClient(policy)
			r := NewPolicyTTLReconciler(fc)
			result, err := r.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: "default", Name: "permanent-policy"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			var got v1alpha1.LatticePolicy
			Expect(fc.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "permanent-policy"}, &got)).To(Succeed())
		})
	})
})
