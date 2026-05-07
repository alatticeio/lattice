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
	"github.com/alatticeio/lattice/internal/server/models"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type fakeSnapStore struct {
	saved []*models.NetworkSnapshot
}

func (f *fakeSnapStore) Create(_ context.Context, s *models.NetworkSnapshot) error {
	f.saved = append(f.saved, s)
	return nil
}
func (f *fakeSnapStore) GetByID(_ context.Context, _ string) (*models.NetworkSnapshot, error) {
	return nil, nil
}
func (f *fakeSnapStore) List(_ context.Context, _ string, _, _ time.Time, _ string) ([]*models.NetworkSnapshot, error) {
	return f.saved, nil
}
func (f *fakeSnapStore) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func TestSnapshotController_CapturesOnPolicyChange(t *testing.T) {
	g := NewWithT(t)

	policy := &v1alpha1.LatticePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-frontend",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.LatticePolicySpec{Action: "ALLOW", Network: "default"},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(newSnapTestScheme()).WithObjects(policy).Build()
	snapStore := &fakeSnapStore{}

	r := controller.NewSnapshotController(fakeClient, snapStore, nil)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "test-ns", Name: "allow-frontend"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(snapStore.saved).To(HaveLen(1))
	g.Expect(snapStore.saved[0].TriggerType).To(Equal("policy_change"))
	g.Expect(snapStore.saved[0].Namespace).To(Equal("test-ns"))
}

func newSnapTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	return s
}
