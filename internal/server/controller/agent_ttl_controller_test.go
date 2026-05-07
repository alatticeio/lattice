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

package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/controller"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newTTLTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestAgentTTLReconciler_DeletesExpiredPeer(t *testing.T) {
	g := NewWithT(t)

	expiredPeer := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-expired",
			Namespace: "test-ns",
			Labels:    map[string]string{"lattice.io/agent-managed": "true"},
			Annotations: map[string]string{
				"lattice.io/agent-expires-at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(expiredPeer).
		Build()

	r := controller.NewAgentTTLReconciler(fakeClient)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "test-ns", Name: "agent-expired"},
	})
	g.Expect(err).ToNot(HaveOccurred())

	var peer v1alpha1.LatticePeer
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "test-ns", Name: "agent-expired"}, &peer)
	g.Expect(apierrors.IsNotFound(getErr)).To(BeTrue(), "expired peer should be deleted")
}

func TestAgentTTLReconciler_KeepsLivePeer(t *testing.T) {
	g := NewWithT(t)

	livePeer := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-live",
			Namespace: "test-ns",
			Labels:    map[string]string{"lattice.io/agent-managed": "true"},
			Annotations: map[string]string{
				"lattice.io/agent-expires-at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(newTTLTestScheme()).WithObjects(livePeer).Build()
	r := controller.NewAgentTTLReconciler(fakeClient)
	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "test-ns", Name: "agent-live"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeNumerically(">", 0), "should requeue before expiry")
}

func TestAgentTTLReconciler_NoTTL_NoOp(t *testing.T) {
	g := NewWithT(t)

	peer := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-no-ttl",
			Namespace: "test-ns",
			Labels:    map[string]string{"lattice.io/agent-managed": "true"},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(newTTLTestScheme()).WithObjects(peer).Build()
	r := controller.NewAgentTTLReconciler(fakeClient)
	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "test-ns", Name: "agent-no-ttl"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

	var gotPeer v1alpha1.LatticePeer
	g.Expect(fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "test-ns", Name: "agent-no-ttl"}, &gotPeer)).To(Succeed())
}
