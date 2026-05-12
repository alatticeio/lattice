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

package seed

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const seedLabel = "lattice.io/is-seed"

// Options controls the volume of seed data injected into a new workspace.
type Options struct {
	VirtualNodes int // default 8
	HistoryDays  int // default 7
	AuditEntries int // default 20
}

// DefaultOptions returns the standard seed volume.
func DefaultOptions() Options {
	return Options{
		VirtualNodes: 8,
		HistoryDays:  7,
		AuditEntries: 20,
	}
}

// Injector injects seed data into a workspace.
type Injector struct {
	store     store.Store
	k8sClient client.Client // optional; skips virtual peer injection when nil
}

// NewInjector creates a new Injector backed by the given store.
// Pass a non-nil k8sClient to enable virtual peer (node) injection.
func NewInjector(s store.Store, k8s client.Client) *Injector {
	return &Injector{store: s, k8sClient: k8s}
}

// Inject writes seed audit logs, policies, alerts, and virtual peers into workspaceID.
// All records are tagged so they can be cleared later.
func (inj *Injector) Inject(ctx context.Context, workspaceID string, opts Options) error {
	if err := inj.injectAuditLogs(ctx, workspaceID, opts); err != nil {
		return fmt.Errorf("seed audit logs: %w", err)
	}
	if err := inj.injectPolicies(ctx, workspaceID); err != nil {
		return fmt.Errorf("seed policies: %w", err)
	}
	if err := inj.injectAlerts(ctx, workspaceID); err != nil {
		return fmt.Errorf("seed alerts: %w", err)
	}
	if inj.k8sClient != nil {
		ws, err := inj.store.Workspaces().GetByID(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("seed peers: get workspace: %w", err)
		}
		if err := inj.injectVirtualPeers(ctx, ws.Namespace, opts); err != nil {
			return fmt.Errorf("seed peers: %w", err)
		}
	}
	return nil
}

// Clear removes all seed records from the given workspace.
func (inj *Injector) Clear(ctx context.Context, workspaceID string) error {
	if inj.k8sClient != nil {
		ws, err := inj.store.Workspaces().GetByID(ctx, workspaceID)
		if err == nil {
			_ = inj.k8sClient.DeleteAllOf(ctx, &v1alpha1.LatticePeer{},
				client.InNamespace(ws.Namespace),
				client.MatchingLabels{seedLabel: "true"},
			)
		}
	}
	return inj.store.Seed().Clear(ctx, workspaceID)
}

func (inj *Injector) injectAuditLogs(ctx context.Context, workspaceID string, opts Options) error {
	now := time.Now()
	entries := opts.AuditEntries
	if entries <= 0 {
		entries = 20
	}
	historyDays := opts.HistoryDays
	if historyDays <= 0 {
		historyDays = 7
	}
	windowSecs := int64(historyDays) * 24 * 3600

	actions := []struct{ action, resource, scope string }{
		{"CREATE", "policy", "policy: allow-web → allow-db"},
		{"UPDATE", "policy", "policy: deny-all → action: DENY"},
		{"CREATE", "member", "member: alice@example.com → role: editor"},
		{"DELETE", "peer", "peer: node-beijing-01"},
		{"CREATE", "token", "token: deploy-token"},
		{"UPDATE", "workspace", "displayName: My Workspace"},
		{"INVITE", "member", "email: bob@example.com"},
		{"CREATE", "policy", "policy: allow-monitoring"},
		{"UPDATE", "member", "member: carol@example.com → role: viewer"},
		{"DELETE", "token", "token: old-token"},
	}

	logs := make([]*models.AuditLog, 0, entries)
	for i := range entries {
		a := actions[i%len(actions)]
		// spread timestamps randomly across the history window, newest first
		offset := time.Duration(rand.Int64N(windowSecs)) * time.Second
		t := now.Add(-offset)
		logs = append(logs, &models.AuditLog{
			ID:           uuid.NewString(),
			CreatedAt:    t,
			UserID:       "seed-user",
			UserName:     "demo-admin",
			UserEmail:    "admin@demo.lattice.io",
			UserIP:       "10.0.0.1",
			WorkspaceID:  workspaceID,
			Action:       a.action,
			Resource:     a.resource,
			ResourceID:   uuid.NewString(),
			ResourceName: fmt.Sprintf("seed-%s-%d", a.resource, i),
			Scope:        a.scope,
			Status:       "success",
			StatusCode:   200,
			IsSeed:       true,
		})
	}
	return inj.store.AuditLogs().BatchCreate(ctx, logs)
}

func (inj *Injector) injectPolicies(ctx context.Context, workspaceID string) error {
	policies := []*models.Policy{
		{
			WorkspaceID: workspaceID,
			Name:        "seed-allow-web-to-db",
			Description: "Allow web tier to reach database tier",
			Action:      "ALLOW",
			PolicyTypes: `["Egress"]`,
			Spec:        `{"selector":{"matchLabels":{"app":"web"}},"egress":[{"to":[{"podSelector":{"matchLabels":{"app":"db"}}}]}]}`,
			Status:      models.PolicyStatusActive,
			IsSeed:      true,
		},
		{
			WorkspaceID: workspaceID,
			Name:        "seed-allow-monitoring",
			Description: "Allow monitoring scrape from any node",
			Action:      "ALLOW",
			PolicyTypes: `["Ingress"]`,
			Spec:        `{"selector":{},"ingress":[{"from":[{"podSelector":{"matchLabels":{"app":"prometheus"}}}],"ports":[{"port":9090}]}]}`,
			Status:      models.PolicyStatusActive,
			IsSeed:      true,
		},
		{
			WorkspaceID: workspaceID,
			Name:        "seed-deny-external",
			Description: "Block inbound traffic from outside the mesh",
			Action:      "DENY",
			PolicyTypes: `["Ingress"]`,
			Spec:        `{"selector":{},"ingress":[]}`,
			Status:      models.PolicyStatusActive,
			IsSeed:      true,
		},
	}
	for _, p := range policies {
		p.ID = uuid.NewString()
		if err := inj.store.Policies().Create(ctx, p); err != nil {
			return fmt.Errorf("create policy %q: %w", p.Name, err)
		}
	}
	return nil
}

func (inj *Injector) injectVirtualPeers(ctx context.Context, namespace string, opts Options) error {
	type nodeSpec struct {
		name     string
		display  string
		platform string
		region   string
	}
	count := opts.VirtualNodes
	if count <= 0 {
		count = 8
	}
	allNodes := []nodeSpec{
		{"seed-beijing-01", "Beijing Node 01", "linux", "cn-north-1"},
		{"seed-beijing-02", "Beijing Node 02", "linux", "cn-north-1"},
		{"seed-shanghai-01", "Shanghai Node 01", "linux", "cn-east-2"},
		{"seed-shanghai-02", "Shanghai Node 02", "linux", "cn-east-2"},
		{"seed-guangzhou-01", "Guangzhou Node 01", "linux", "cn-south-1"},
		{"seed-shenzhen-01", "Shenzhen MacBook", "darwin", "cn-south-1"},
		{"seed-chengdu-01", "Chengdu Node 01", "linux", "cn-southwest-1"},
		{"seed-wuhan-01", "Wuhan Node 01", "linux", "cn-central-1"},
	}
	if count > len(allNodes) {
		count = len(allNodes)
	}
	for _, n := range allNodes[:count] {
		peer := &v1alpha1.LatticePeer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.name,
				Namespace: namespace,
				Labels: map[string]string{
					seedLabel: "true",
					"region":  n.region,
				},
				Annotations: map[string]string{
					"lattice.io/display-name": n.display,
				},
			},
			Spec: v1alpha1.LatticePeerSpec{
				AppId:    n.name,
				Platform: n.platform,
			},
		}
		if err := inj.k8sClient.Create(ctx, peer); err != nil {
			return fmt.Errorf("create peer %s: %w", n.name, err)
		}
	}
	return nil
}

func (inj *Injector) injectAlerts(ctx context.Context, workspaceID string) error {
	now := time.Now()
	ended := now.Add(-2 * time.Hour)

	alerts := []*models.AlertHistory{
		{
			RuleID:      "seed-rule-offline",
			WorkspaceID: workspaceID,
			Status:      "resolved",
			Severity:    "warning",
			Labels:      `{"node":"node-shanghai-02"}`,
			Value:       0,
			Message:     "Node node-shanghai-02 went offline for 5 minutes",
			StartedAt:   now.Add(-3 * time.Hour),
			EndedAt:     &ended,
			Notified:    true,
			IsSeed:      true,
		},
		{
			RuleID:      "seed-rule-latency",
			WorkspaceID: workspaceID,
			Status:      "resolved",
			Severity:    "info",
			Labels:      `{"node":"node-guangzhou-01"}`,
			Value:       280.5,
			Message:     "RTT to node-guangzhou-01 exceeded 250ms",
			StartedAt:   now.Add(-26 * time.Hour),
			EndedAt:     &ended,
			Notified:    true,
			IsSeed:      true,
		},
	}
	for _, a := range alerts {
		if err := inj.store.Alerts().CreateAlertHistory(ctx, a); err != nil {
			return err
		}
	}
	return nil
}
