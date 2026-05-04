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

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/alatticeio/lattice/internal/server/vo"
)

// resolveWorkspaceID looks up the workspace UUID for a given K8s namespace string.
// The CLI uses namespace as the user-visible identifier; the HTTP API requires the UUID.
func (c *Client) resolveWorkspaceID(namespace string) (string, error) {
	var list []vo.WorkspaceVo
	if err := c.do(context.Background(), http.MethodGet, "/api/v1/workspaces/list", "", nil, &list); err != nil {
		return "", fmt.Errorf("listing workspaces: %w", err)
	}
	for _, ws := range list {
		if ws.Namespace == namespace {
			return ws.ID, nil
		}
	}
	return "", fmt.Errorf("workspace with namespace %q not found", namespace)
}

// ── workspace ─────────────────────────────────────────────────────────────────

// AddWorkspace creates a workspace and prints the result.
func (c *Client) AddWorkspace(slug, namespace, displayName string) error {
	var ws vo.WorkspaceVo
	err := c.do(context.Background(), http.MethodPost, "/api/v1/workspaces/add", "", map[string]string{
		"slug":        slug,
		"namespace":   namespace,
		"displayName": displayName,
	}, &ws)
	if err != nil {
		return err
	}
	fmt.Printf("workspace created\n")
	fmt.Printf("  name:      %s\n", ws.Slug)
	fmt.Printf("  namespace: %s\n", ws.Namespace)
	fmt.Printf("  status:    %s\n", ws.Status)
	fmt.Printf("\nUse -n %s for token/policy commands targeting this workspace.\n", ws.Namespace)
	return nil
}

// RemoveWorkspace deletes a workspace identified by its K8s namespace.
func (c *Client) RemoveWorkspace(namespace string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	if err := c.do(context.Background(), http.MethodDelete, "/api/v1/workspaces/"+wsID, "", nil, nil); err != nil {
		return err
	}
	fmt.Printf("workspace %q removed\n", namespace)
	return nil
}

// ListWorkspaces prints all workspaces as a table.
func (c *Client) ListWorkspaces() error {
	var list []vo.WorkspaceVo
	if err := c.do(context.Background(), http.MethodGet, "/api/v1/workspaces/list", "", nil, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no workspaces found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tNAMESPACE\tDISPLAY-NAME\tNODES\tSTATUS") //nolint:errcheck
	for _, ws := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", //nolint:errcheck
			ws.Slug, ws.Namespace, ws.DisplayName, ws.NodeCount, ws.Status)
	}
	return w.Flush()
}

// ── policy ────────────────────────────────────────────────────────────────────

// AddPolicy creates or updates a network policy in the given workspace namespace.
func (c *Client) AddPolicy(namespace, name, action, description string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	var p vo.PolicyVo
	err = c.do(context.Background(), http.MethodPost, "/api/v1/policies/create", wsID, map[string]string{
		"name":        name,
		"namespace":   namespace,
		"action":      action,
		"description": description,
	}, &p)
	if err != nil {
		return err
	}
	fmt.Printf("policy %q applied\n", p.Name)
	fmt.Printf("  action:  %s\n", p.Action)
	fmt.Printf("  types:   %s\n", strings.Join(p.PolicyTypes, ", "))
	return nil
}

// AllowAll creates a full-mesh ALLOW policy for the given workspace namespace.
func (c *Client) AllowAll(namespace string) error {
	return c.AddPolicy(namespace, "allow-all", "ALLOW", "Full mesh — allow all peer traffic")
}

// RemovePolicy deletes a policy by name.
func (c *Client) RemovePolicy(namespace, name string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	if err := c.do(context.Background(), http.MethodDelete, "/api/v1/policies/"+name, wsID, nil, nil); err != nil {
		return err
	}
	fmt.Printf("policy %q removed from %s\n", name, namespace)
	return nil
}

// ListPolicies prints all policies in the given namespace as a table.
func (c *Client) ListPolicies(namespace string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	var list []vo.PolicyVo
	if err := c.do(context.Background(), http.MethodGet, "/api/v1/policies/list", wsID, nil, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Printf("no policies in namespace %q\n", namespace)
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tACTION\tTYPES\tDESCRIPTION") //nolint:errcheck
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", //nolint:errcheck
			p.Name, p.Action, strings.Join(p.PolicyTypes, ","), p.Description)
	}
	return w.Flush()
}

// ── token ─────────────────────────────────────────────────────────────────────

// CreateToken creates an enrollment token in the given workspace namespace.
func (c *Client) CreateToken(namespace, name, expiry string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	var result map[string]string
	err = c.do(context.Background(), http.MethodPost, "/api/v1/token/generate", wsID, map[string]string{
		"name":   name,
		"expiry": expiry,
	}, &result)
	if err != nil {
		return err
	}
	fmt.Printf("Token Created: %s\n", result["token"])
	return nil
}

// ListTokens prints all enrollment tokens filtered by namespace.
func (c *Client) ListTokens(namespace string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	var list []*vo.TokenVo
	if err := c.do(context.Background(), http.MethodGet, "/api/v1/token/list", wsID, nil, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no tokens found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TOKEN\tNAMESPACE\tLIMIT\tEXPIRY") //nolint:errcheck
	for _, t := range list {
		expiry := "never"
		if !t.Expiry.IsZero() {
			expiry = t.Expiry.Time.Format("2006-01-02 15:04")
		}
		limit := fmt.Sprintf("%d", t.UsageLimit)
		if t.UsageLimit == 0 {
			limit = "unlimited"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Token, t.Namespace, limit, expiry) //nolint:errcheck
	}
	return w.Flush()
}

// RemoveToken revokes an enrollment token by its value.
func (c *Client) RemoveToken(namespace, token string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	if err := c.do(context.Background(), http.MethodDelete, "/api/v1/token/"+token, wsID, nil, nil); err != nil {
		return err
	}
	fmt.Printf("token %q revoked\n", token)
	return nil
}

// ── peer ──────────────────────────────────────────────────────────────────────

type peerRow struct {
	Name    string            `json:"name"`
	AppID   string            `json:"app_id"`
	IP      string            `json:"ip"`
	Network string            `json:"network"`
	Phase   string            `json:"phase"`
	Labels  map[string]string `json:"labels"`
}

// ListPeers prints all LatticePeers in the given namespace.
func (c *Client) ListPeers(namespace string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	var list []peerRow
	if err := c.do(context.Background(), http.MethodGet, "/api/v1/peers/list", wsID, nil, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Printf("no peers in namespace %q\n", namespace)
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tAPP-ID\tIP\tNETWORK\tPHASE") //nolint:errcheck
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", //nolint:errcheck
			p.Name, p.AppID, p.IP, p.Network, p.Phase)
	}
	return w.Flush()
}

// PeerLabel merges labels into a LatticePeer's metadata.
func (c *Client) PeerLabel(namespace, peerName string, labels map[string]string) error {
	wsID, err := c.resolveWorkspaceID(namespace)
	if err != nil {
		return err
	}
	var result struct {
		Peer   string            `json:"peer"`
		Labels map[string]string `json:"labels"`
	}
	err = c.do(context.Background(), http.MethodPut, "/api/v1/peers/update", wsID, map[string]any{
		"peer_name": peerName,
		"labels":    labels,
	}, &result)
	if err != nil {
		return err
	}
	fmt.Printf("labels updated on peer %q\n", result.Peer)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE") //nolint:errcheck
	for k, v := range result.Labels {
		fmt.Fprintf(w, "%s\t%s\n", k, v) //nolint:errcheck
	}
	return w.Flush()
}
