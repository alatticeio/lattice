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

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	agentExpiresAtAnnotation = "lattice.io/agent-expires-at" // RFC3339
	agentTypeLabel           = "lattice.io/agent-type"
	agentManagedLabel        = "lattice.io/agent-managed" // "true" marks TTL-managed peers
)

// Policy preset names.
const (
	PresetSandboxed   = "sandboxed"   // egress only to tool services; no ingress
	PresetCoordinator = "coordinator" // accepts ingress from same-workspace agents
	PresetIsolated    = "isolated"    // no network access except explicit whitelist
)

// AgentEnrollRequest carries all parameters for agent enrollment.
type AgentEnrollRequest struct {
	AgentName    string        // human-readable name, used in peer name
	AgentType    string        // e.g. "code-executor", "coordinator"
	Namespace    string        // K8s namespace (= workspace namespace)
	TTL          time.Duration // 0 means no expiry
	PolicyPreset string        // sandboxed | coordinator | isolated
}

// AgentEnrollResponse is returned to the enrolling agent.
type AgentEnrollResponse struct {
	PeerName        string    `json:"peerName"`
	OverlayIP       string    `json:"overlayIP,omitempty"` // populated once IPAM assigns it
	EnrollmentToken string    `json:"enrollmentToken"`
	ExpiresAt       time.Time `json:"expiresAt,omitempty"`
}

// AgentEnrollService manages agent lifecycle enrollment.
type AgentEnrollService interface {
	Enroll(ctx context.Context, req AgentEnrollRequest) (*AgentEnrollResponse, error)
	Revoke(ctx context.Context, namespace, peerName string) error
}

type agentEnrollService struct {
	logger *log.Logger
	client client.Client
}

// NewAgentEnrollService creates an AgentEnrollService backed by the resource.Client.
func NewAgentEnrollService(k8s *resource.Client) AgentEnrollService {
	return &agentEnrollService{
		logger: log.GetLogger("agent-enroll"),
		client: k8s.GetClient(),
	}
}

// NewAgentEnrollServiceWithClient creates an AgentEnrollService from a raw client (for testing).
func NewAgentEnrollServiceWithClient(c client.Client) AgentEnrollService {
	return &agentEnrollService{
		logger: log.GetLogger("agent-enroll"),
		client: c,
	}
}

func (s *agentEnrollService) Enroll(ctx context.Context, req AgentEnrollRequest) (*AgentEnrollResponse, error) {
	peerName := fmt.Sprintf("agent-%s", req.AgentName)

	annotations := map[string]string{}
	var expiresAt time.Time
	if req.TTL > 0 {
		expiresAt = time.Now().Add(req.TTL)
		annotations[agentExpiresAtAnnotation] = expiresAt.UTC().Format(time.RFC3339)
	}

	peer := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      peerName,
			Namespace: req.Namespace,
			Labels: map[string]string{
				agentTypeLabel:    req.AgentType,
				agentManagedLabel: "true",
			},
			Annotations: annotations,
		},
		Spec: v1alpha1.LatticePeerSpec{
			AppId:    generateAppID(),
			Platform: "agent",
		},
	}
	if err := s.client.Create(ctx, peer); err != nil {
		return nil, fmt.Errorf("create peer: %w", err)
	}
	s.logger.Info("agent peer created", "name", peerName, "namespace", req.Namespace)

	if err := s.applyPreset(ctx, req.Namespace, peerName, req.AgentType, req.PolicyPreset); err != nil {
		s.logger.Warn("preset application failed (peer still created)", "err", err)
	}

	token, tokenName, err := s.createEnrollmentToken(ctx, req.Namespace, peerName, req.TTL)
	if err != nil {
		return nil, fmt.Errorf("create enrollment token: %w", err)
	}
	s.logger.Info("enrollment token created", "tokenName", tokenName)

	return &AgentEnrollResponse{
		PeerName:        peerName,
		EnrollmentToken: token,
		ExpiresAt:       expiresAt,
	}, nil
}

func (s *agentEnrollService) Revoke(ctx context.Context, namespace, peerName string) error {
	var peer v1alpha1.LatticePeer
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: peerName}, &peer); err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}
	if err := s.client.Delete(ctx, &peer); err != nil {
		return fmt.Errorf("delete peer: %w", err)
	}
	s.logger.Info("agent peer revoked", "name", peerName)
	return nil
}

func (s *agentEnrollService) createEnrollmentToken(ctx context.Context, namespace, peerName string, ttl time.Duration) (string, string, error) {
	rawToken := make([]byte, 16)
	if _, err := rand.Read(rawToken); err != nil {
		return "", "", err
	}
	token := "lt-" + hex.EncodeToString(rawToken)
	tokenName := fmt.Sprintf("token-%s", peerName)

	expiry := metav1.NewTime(time.Now().Add(24 * time.Hour))
	if ttl > 0 {
		expiry = metav1.NewTime(time.Now().Add(ttl))
	}

	tokenObj := &v1alpha1.LatticeEnrollmentToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenName,
			Namespace: namespace,
		},
		Spec: v1alpha1.LatticeEnrollmentTokenSpec{
			Token:      token,
			Namespace:  namespace,
			UsageLimit: 1,
			Expiry:     expiry,
			BoundPeers: []string{peerName},
		},
	}
	if err := s.client.Create(ctx, tokenObj); err != nil {
		return "", "", err
	}
	return token, tokenName, nil
}

// applyPreset creates LatticePolicy CRDs based on the preset name.
func (s *agentEnrollService) applyPreset(ctx context.Context, namespace, peerName, agentType, preset string) error {
	switch preset {
	case PresetSandboxed:
		policy := &v1alpha1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("agent-deny-ingress-%s", peerName),
				Namespace: namespace,
			},
			Spec: v1alpha1.LatticePolicySpec{
				Action: "DENY",
				PeerSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{agentTypeLabel: agentType},
				},
			},
		}
		return s.client.Create(ctx, policy)

	case PresetCoordinator:
		policy := &v1alpha1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("agent-allow-ingress-%s", peerName),
				Namespace: namespace,
			},
			Spec: v1alpha1.LatticePolicySpec{
				Action: "ALLOW",
				PeerSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{agentTypeLabel: agentType},
				},
				Ingress: []v1alpha1.IngressRule{
					{
						From: []v1alpha1.PeerSelection{
							{
								PeerSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{agentManagedLabel: "true"},
								},
							},
						},
					},
				},
			},
		}
		return s.client.Create(ctx, policy)

	case PresetIsolated:
		policy := &v1alpha1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("agent-deny-all-%s", peerName),
				Namespace: namespace,
			},
			Spec: v1alpha1.LatticePolicySpec{
				Action: "DENY",
				PeerSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{agentTypeLabel: agentType},
				},
			},
		}
		return s.client.Create(ctx, policy)

	default:
		return fmt.Errorf("unknown policy preset: %q (valid: sandboxed, coordinator, isolated)", preset)
	}
}

func generateAppID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "agent-" + hex.EncodeToString(b)
}
