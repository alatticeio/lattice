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

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAgentIdentityDefaults(t *testing.T) {
	now := metav1.NewTime(time.Now().Add(24 * time.Hour))
	ai := AgentIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "ws-a"},
		Spec: AgentIdentitySpec{
			PeerRef:           "peer-test",
			AllowedTools:      []string{"list_peers"},
			AllowedNamespaces: []string{"ws-a"},
			Sandbox:           SandboxPod,
			AuditLevel:        AuditLevelFull,
			EnforcementMode:   EnforcementEnforce,
			ExpiresAt:         &now,
		},
	}
	if ai.Spec.PeerRef == "" {
		t.Error("expected PeerRef to be set")
	}
	if len(ai.Spec.AllowedTools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(ai.Spec.AllowedTools))
	}
}

func TestAgentIdentityPhase(t *testing.T) {
	ai := AgentIdentity{}
	ai.Status.Phase = AgentPhaseActive
	if ai.Status.Phase != AgentPhaseActive {
		t.Errorf("expected Active phase")
	}
}
