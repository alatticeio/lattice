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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/alatticeio/lattice/api/v1alpha1"
)

var _ = Describe("NetworkPolicyReconciler", func() {
	ns := "default"

	It("writes targetNodes and ruleCount to status", func() {
		// Create two peers with labels
		peer1 := &v1alpha1.LatticePeer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "peer-status-1",
				Namespace: ns,
				Labels:    map[string]string{"role": "backend"},
			},
			Spec: v1alpha1.LatticePeerSpec{},
		}
		peer2 := &v1alpha1.LatticePeer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "peer-status-2",
				Namespace: ns,
				Labels:    map[string]string{"role": "backend"},
			},
			Spec: v1alpha1.LatticePeerSpec{},
		}
		Expect(k8sClient.Create(ctx, peer1)).To(Succeed())
		Expect(k8sClient.Create(ctx, peer2)).To(Succeed())

		// Create a policy targeting role=backend with 2 ingress rules
		policy := &v1alpha1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-status-policy",
				Namespace: ns,
			},
			Spec: v1alpha1.LatticePolicySpec{
				Network: "test-net",
				PeerSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"role": "backend"},
				},
				Action: "ALLOW",
				Ingress: []v1alpha1.IngressRule{
					{From: []v1alpha1.PeerSelection{{PeerSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"role": "frontend"}}}}},
					{From: []v1alpha1.PeerSelection{{IPBlock: &v1alpha1.IPBlock{CIDR: "10.0.0.0/8"}}}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, policy)
			_ = k8sClient.Delete(ctx, peer1)
			_ = k8sClient.Delete(ctx, peer2)
		})

		// Instantiate and invoke reconciler directly (no manager running in envtest suite)
		reconciler := &NetworkPolicyReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(policy),
		})
		Expect(err).NotTo(HaveOccurred())

		var updated v1alpha1.LatticePolicy
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), &updated)).To(Succeed())
		Expect(updated.Status.TargetNodes).To(Equal(2))
		Expect(updated.Status.RuleCount).To(Equal(2))
	})
})
