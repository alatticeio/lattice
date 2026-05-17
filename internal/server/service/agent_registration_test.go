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

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/service"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestK8sClient() *fake.ClientBuilder {
	s := runtime.NewScheme()
	return fake.NewClientBuilder().WithScheme(newTestScheme()).WithScheme(s)
}

func TestCreateEnrollmentToken_ReturnsToken(t *testing.T) {
	st := newTestDB(t)
	k8s := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	svc := service.NewAgentRegistrationService("test-secret", st, k8s)

	tok, err := svc.CreateEnrollmentToken(context.Background(), service.EnrollmentTokenRequest{
		AllowedNamespace: "ws-a",
		AllowedTools:     []string{"list_peers", "check_connectivity"},
		TTL:              time.Hour,
		CreatedBy:        "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok.Token) < 32 {
		t.Errorf("token too short: %q", tok.Token)
	}
	if tok.ExpiresAt.Before(time.Now()) {
		t.Error("token should not already be expired")
	}
}

func TestRegisterAgent_InvalidToken(t *testing.T) {
	st := newTestDB(t)
	k8s := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	svc := service.NewAgentRegistrationService("test-secret", st, k8s)

	_, err := svc.RegisterAgent(context.Background(), service.AgentRegisterRequest{
		EnrollmentToken: "invalid-token",
		AgentName:       "agent-test",
		PublicKey:       "fake-wg-pubkey",
	})
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestRegisterAgent_ValidToken(t *testing.T) {
	st := newTestDB(t)
	k8s := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	svc := service.NewAgentRegistrationService("test-secret", st, k8s)

	// Create a valid enrollment token first
	tok, err := svc.CreateEnrollmentToken(context.Background(), service.EnrollmentTokenRequest{
		AllowedNamespace: "ws-a",
		AllowedTools:     []string{"list_peers"},
		TTL:              time.Hour,
		CreatedBy:        "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error creating token: %v", err)
	}

	// Register an agent with the token
	resp, err := svc.RegisterAgent(context.Background(), service.AgentRegisterRequest{
		EnrollmentToken: tok.Token,
		AgentName:       "agent-001",
		PublicKey:       "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	})
	if err != nil {
		t.Fatalf("unexpected error registering agent: %v", err)
	}
	if len(resp.JWT) == 0 {
		t.Error("expected non-empty JWT")
	}
	if resp.AgentIdentityName != "agent-001" {
		t.Errorf("expected AgentIdentityName=agent-001, got %q", resp.AgentIdentityName)
	}
}

func TestRevokeAgent_SetsRevokedPhase(t *testing.T) {
	identity := &latticev1.AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-001",
			Namespace: "ws-a",
		},
		Status: latticev1.AgentIdentityStatus{
			Phase: latticev1.AgentPhaseActive,
		},
	}
	k8s := fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithObjects(identity).
		WithStatusSubresource(&latticev1.AgentIdentity{}).
		Build()
	// Pre-populate status (fake client requires explicit status creation when WithStatusSubresource is used).
	identity.Status.Phase = latticev1.AgentPhaseActive
	_ = k8s.Status().Update(context.Background(), identity)

	st := newTestDB(t)
	svc := service.NewAgentRegistrationService("test-secret", st, k8s)

	err := svc.RevokeAgent(context.Background(), "ws-a", "agent-001")
	if err != nil {
		t.Fatalf("unexpected error revoking agent: %v", err)
	}

	got := &latticev1.AgentIdentity{}
	if err := k8s.Get(context.Background(), client.ObjectKey{Namespace: "ws-a", Name: "agent-001"}, got); err != nil {
		t.Fatalf("AgentIdentity should still exist after revocation: %v", err)
	}
	if got.Status.Phase != latticev1.AgentPhaseRevoked {
		t.Errorf("expected phase Revoked, got %q", got.Status.Phase)
	}
}

func TestRevokeAgent_NotFound_IsNoop(t *testing.T) {
	k8s := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	st := newTestDB(t)
	svc := service.NewAgentRegistrationService("test-secret", st, k8s)

	err := svc.RevokeAgent(context.Background(), "ws-a", "nonexistent")
	if err != nil {
		t.Errorf("expected no error for missing AgentIdentity, got: %v", err)
	}
}

func TestRegisterAgent_TokenAlreadyUsed(t *testing.T) {
	st := newTestDB(t)
	k8s := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	svc := service.NewAgentRegistrationService("test-secret", st, k8s)

	tok, err := svc.CreateEnrollmentToken(context.Background(), service.EnrollmentTokenRequest{
		AllowedNamespace: "ws-b",
		AllowedTools:     []string{"list_peers"},
		TTL:              time.Hour,
		CreatedBy:        "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First registration succeeds
	_, err = svc.RegisterAgent(context.Background(), service.AgentRegisterRequest{
		EnrollmentToken: tok.Token,
		AgentName:       "agent-first",
		PublicKey:       "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	})
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration with same token must fail
	_, err = svc.RegisterAgent(context.Background(), service.AgentRegisterRequest{
		EnrollmentToken: tok.Token,
		AgentName:       "agent-second",
		PublicKey:       "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=",
	})
	if err == nil {
		t.Error("expected error for already-used token")
	}
}
