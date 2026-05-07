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

package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/service"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestAgentEnroll_CreatesPresetsAndToken(t *testing.T) {
	g := NewWithT(t)

	fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	svc := service.NewAgentEnrollServiceWithClient(fakeClient)

	resp, err := svc.Enroll(context.Background(), service.AgentEnrollRequest{
		AgentName:    "code-executor-001",
		AgentType:    "code-executor",
		Namespace:    "test-ns",
		TTL:          time.Hour,
		PolicyPreset: "sandboxed",
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp.PeerName).To(Equal("agent-code-executor-001"))
	g.Expect(resp.EnrollmentToken).To(HavePrefix("lt-"))
	g.Expect(resp.ExpiresAt).To(BeTemporally(">", time.Now()))
}

func TestAgentEnroll_Revoke(t *testing.T) {
	g := NewWithT(t)

	fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	svc := service.NewAgentEnrollServiceWithClient(fakeClient)

	resp, err := svc.Enroll(context.Background(), service.AgentEnrollRequest{
		AgentName:    "test-agent",
		AgentType:    "code-executor",
		Namespace:    "test-ns",
		TTL:          time.Hour,
		PolicyPreset: "sandboxed",
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = svc.Revoke(context.Background(), "test-ns", resp.PeerName)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAgentEnroll_AllPresets(t *testing.T) {
	presets := []string{"sandboxed", "coordinator", "isolated"}
	for _, preset := range presets {
		t.Run(preset, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
			svc := service.NewAgentEnrollServiceWithClient(fakeClient)

			resp, err := svc.Enroll(context.Background(), service.AgentEnrollRequest{
				AgentName:    "agent-" + preset,
				AgentType:    "test",
				Namespace:    "test-ns",
				TTL:          time.Hour,
				PolicyPreset: preset,
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp.EnrollmentToken).To(HavePrefix("lt-"))
		})
	}
}
