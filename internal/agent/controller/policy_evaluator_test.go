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

	"github.com/alatticeio/lattice/internal/agent/infra"
)

var _ = Describe("PolicyEvaluator", func() {
	var (
		evaluator PolicyEvaluator
		ctx       context.Context
		current   *infra.Peer
		network   *infra.Network
	)

	BeforeEach(func() {
		evaluator = NewPolicyEvaluator()
		ctx = context.Background()
		current = &infra.Peer{Name: "backend-1", Address: strPtr("10.0.0.1")}
		network = &infra.Network{
			Peers: []*infra.Peer{
				{Name: "frontend-1", Address: strPtr("10.0.0.2")},
				{Name: "frontend-2", Address: strPtr("10.0.0.3")},
				{Name: "backend-1", Address: strPtr("10.0.0.1")},
			},
		}
	})

	Describe("no policies", func() {
		It("returns only default-deny rules", func() {
			result, err := evaluator.Evaluate(ctx, current, network, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Ingress).To(HaveLen(1))
			Expect(result.Ingress[0].Action).To(Equal("DROP"))
			Expect(result.Ingress[0].Peers).To(BeEmpty())
			Expect(result.Egress).To(HaveLen(1))
			Expect(result.Egress[0].Action).To(Equal("DROP"))
		})
	})

	Describe("single ALLOW policy", func() {
		It("emits ACCEPT rules for matched peers", func() {
			policies := []*infra.Policy{
				{
					PolicyName: "allow-frontend",
					Action:     "ALLOW",
					Ingress: []*infra.Rule{
						{PeerNames: []string{"frontend-1"}, Action: "ALLOW"},
					},
				},
			}
			result, err := evaluator.Evaluate(ctx, current, network, policies)
			Expect(err).NotTo(HaveOccurred())
			// Should have 1 ACCEPT rule + 1 default-deny DROP
			acceptRules := filterByAction(result.Ingress, "ACCEPT")
			Expect(acceptRules).To(HaveLen(1))
			Expect(acceptRules[0].Peers).To(ContainElement("10.0.0.2"))
			// Last rule is default-deny
			last := result.Ingress[len(result.Ingress)-1]
			Expect(last.Action).To(Equal("DROP"))
			Expect(last.Peers).To(BeEmpty())
		})
	})

	Describe("single DENY policy", func() {
		It("emits DROP rules for matched peers", func() {
			policies := []*infra.Policy{
				{
					PolicyName: "deny-frontend",
					Action:     "DENY",
					Ingress: []*infra.Rule{
						{PeerNames: []string{"frontend-1"}, Action: "DENY"},
					},
				},
			}
			result, err := evaluator.Evaluate(ctx, current, network, policies)
			Expect(err).NotTo(HaveOccurred())
			dropRules := filterByAction(result.Ingress, "DROP")
			// At least 1 explicit DROP + 1 default-deny DROP
			Expect(len(dropRules)).To(BeNumerically(">=", 2))
			explicitDrops := filterByActionWithPeers(result.Ingress, "DROP")
			Expect(explicitDrops).To(HaveLen(1))
			Expect(explicitDrops[0].Peers).To(ContainElement("10.0.0.2"))
		})
	})

	Describe("ALLOW-priority conflict", func() {
		It("ALLOW wins when two policies disagree on the same IP", func() {
			policies := []*infra.Policy{
				{
					PolicyName: "deny-policy",
					Action:     "DENY",
					Ingress:    []*infra.Rule{{PeerNames: []string{"frontend-1"}, Action: "DENY"}},
				},
				{
					PolicyName: "allow-policy",
					Action:     "ALLOW",
					Ingress:    []*infra.Rule{{PeerNames: []string{"frontend-1"}, Action: "ALLOW"}},
				},
			}
			result, err := evaluator.Evaluate(ctx, current, network, policies)
			Expect(err).NotTo(HaveOccurred())
			acceptRules := filterByAction(result.Ingress, "ACCEPT")
			Expect(acceptRules).To(HaveLen(1))
			Expect(acceptRules[0].Peers).To(ContainElement("10.0.0.2"))
		})
	})

	Describe("IPBlock CIDR rule", func() {
		It("emits TrafficRule with CIDR in Peers", func() {
			policies := []*infra.Policy{
				{
					PolicyName: "allow-cidr",
					Action:     "ALLOW",
					Ingress:    []*infra.Rule{{CIDRs: []string{"192.168.1.0/24"}, Action: "ALLOW"}},
				},
			}
			result, err := evaluator.Evaluate(ctx, current, network, policies)
			Expect(err).NotTo(HaveOccurred())
			acceptRules := filterByAction(result.Ingress, "ACCEPT")
			Expect(acceptRules).To(HaveLen(1))
			Expect(acceptRules[0].Peers).To(ContainElement("192.168.1.0/24"))
		})
	})

	Describe("port-specific rule", func() {
		It("emits TrafficRule with port and protocol", func() {
			policies := []*infra.Policy{
				{
					PolicyName: "allow-port",
					Action:     "ALLOW",
					Ingress: []*infra.Rule{
						{PeerNames: []string{"frontend-1"}, Protocol: "tcp", Port: 8080, Action: "ALLOW"},
					},
				},
			}
			result, err := evaluator.Evaluate(ctx, current, network, policies)
			Expect(err).NotTo(HaveOccurred())
			acceptRules := filterByAction(result.Ingress, "ACCEPT")
			Expect(acceptRules).To(HaveLen(1))
			Expect(acceptRules[0].Port).To(Equal(8080))
			Expect(acceptRules[0].Protocol).To(Equal("tcp"))
		})
	})

	Describe("current peer skipped", func() {
		It("does not emit rule for current peer's own IP", func() {
			policies := []*infra.Policy{
				{
					PolicyName: "allow-self",
					Action:     "ALLOW",
					Ingress:    []*infra.Rule{{PeerNames: []string{"backend-1"}, Action: "ALLOW"}},
				},
			}
			result, err := evaluator.Evaluate(ctx, current, network, policies)
			Expect(err).NotTo(HaveOccurred())
			for _, tr := range result.Ingress {
				Expect(tr.Peers).NotTo(ContainElement("10.0.0.1"))
			}
		})
	})
})

func strPtr(s string) *string { return &s }

func filterByAction(rules []infra.TrafficRule, action string) []infra.TrafficRule {
	var out []infra.TrafficRule
	for _, r := range rules {
		if r.Action == action {
			out = append(out, r)
		}
	}
	return out
}

func filterByActionWithPeers(rules []infra.TrafficRule, action string) []infra.TrafficRule {
	var out []infra.TrafficRule
	for _, r := range rules {
		if r.Action == action && len(r.Peers) > 0 {
			out = append(out, r)
		}
	}
	return out
}
