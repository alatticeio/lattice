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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestAgentIdentityReconciler_MarksExpired(t *testing.T) {
	g := NewWithT(t)

	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "claude-1",
			Namespace: "default",
		},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:         "claude-1",
			AllowedTools:    []string{"list_peers"},
			EnforcementMode: v1alpha1.EnforcementEnforce,
			ExpiresAt:       &metav1.Time{Time: time.Now().Add(-time.Minute)},
		},
		Status: v1alpha1.AgentIdentityStatus{
			Phase: v1alpha1.AgentPhaseActive,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&v1alpha1.AgentIdentity{}).
		Build()

	r := controller.NewAgentIdentityReconciler(fakeClient)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "claude-1"},
	})
	g.Expect(err).ToNot(HaveOccurred())

	var got v1alpha1.AgentIdentity
	g.Expect(fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claude-1"}, &got)).To(Succeed())
	g.Expect(got.Status.Phase).To(Equal(v1alpha1.AgentPhaseExpired))
}

func TestAgentIdentityReconciler_RequeuesBeforeExpiry(t *testing.T) {
	g := NewWithT(t)

	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "claude-2",
			Namespace: "default",
		},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:      "claude-2",
			AllowedTools: []string{"list_peers"},
			ExpiresAt:    &metav1.Time{Time: time.Now().Add(time.Hour)},
		},
		Status: v1alpha1.AgentIdentityStatus{Phase: v1alpha1.AgentPhaseActive},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&v1alpha1.AgentIdentity{}).
		Build()

	r := controller.NewAgentIdentityReconciler(fakeClient)
	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "claude-2"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeNumerically(">", 0))

	var got v1alpha1.AgentIdentity
	g.Expect(fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claude-2"}, &got)).To(Succeed())
	g.Expect(got.Status.Phase).To(Equal(v1alpha1.AgentPhaseActive), "should not change phase for live identity")
}

func TestAgentIdentityReconciler_NoExpiry_NoOp(t *testing.T) {
	g := NewWithT(t)

	identity := &v1alpha1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "claude-3", Namespace: "default"},
		Spec: v1alpha1.AgentIdentitySpec{
			PeerRef:      "claude-3",
			AllowedTools: []string{"list_peers"},
		},
		Status: v1alpha1.AgentIdentityStatus{Phase: v1alpha1.AgentPhaseActive},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(newTTLTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&v1alpha1.AgentIdentity{}).
		Build()

	r := controller.NewAgentIdentityReconciler(fakeClient)
	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "claude-3"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
}
