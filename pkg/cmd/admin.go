// Copyright 2025 The Wireflow Authors, Inc.
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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"wireflow/management/models"
	"wireflow/management/vo"
)

// call sends a NATS request to "wireflow.signals.service.<method>" and returns
// the raw JSON response body, or an error if the server returned one.
func (c *Client) call(method string, payload any) ([]byte, error) {
	bs, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.client.Request(context.Background(), "wireflow.signals.service", method, bs)
}

// ── workspace ─────────────────────────────────────────────────────────────────

// AddWorkspace creates a workspace and prints the result.
func (c *Client) AddWorkspace(slug, namespace, displayName string) error {
	data, err := c.call("workspace.add", map[string]string{
		"slug":         slug,
		"namespace":    namespace,
		"display_name": displayName,
	})
	if err != nil {
		return err
	}
	var ws vo.WorkspaceVo
	if err = json.Unmarshal(data, &ws); err != nil {
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
	_, err := c.call("workspace.remove", map[string]string{"namespace": namespace})
	if err != nil {
		return err
	}
	fmt.Printf("workspace %q removed\n", namespace)
	return nil
}

// ListWorkspaces prints all workspaces as a table.
func (c *Client) ListWorkspaces() error {
	data, err := c.call("workspace.list", struct{}{})
	if err != nil {
		return err
	}
	var list []vo.WorkspaceVo
	if err = json.Unmarshal(data, &list); err != nil {
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

// AddPolicy creates or updates a network policy.
func (c *Client) AddPolicy(namespace, name, action, description string) error {
	data, err := c.call("policy.add", map[string]string{
		"namespace":   namespace,
		"name":        name,
		"action":      action,
		"description": description,
	})
	if err != nil {
		return err
	}
	var p vo.PolicyVo
	if err = json.Unmarshal(data, &p); err != nil {
		return err
	}
	fmt.Printf("policy %q applied\n", p.Name)
	fmt.Printf("  action:  %s\n", p.Action)
	fmt.Printf("  types:   %s\n", strings.Join(p.PolicyTypes, ", "))
	return nil
}

// AllowAll creates a full-mesh allow-all policy for the given namespace.
func (c *Client) AllowAll(namespace string) error {
	return c.AddPolicy(namespace, "allow-all", "ALLOW", "allow all peer traffic (created by CLI)")
}

// RemovePolicy deletes a policy by name from the given namespace.
func (c *Client) RemovePolicy(namespace, name string) error {
	_, err := c.call("policy.remove", map[string]string{
		"namespace": namespace,
		"name":      name,
	})
	if err != nil {
		return err
	}
	fmt.Printf("policy %q removed from %s\n", name, namespace)
	return nil
}

// ListPolicies prints all policies in the given namespace as a table.
func (c *Client) ListPolicies(namespace string) error {
	data, err := c.call("policy.list", map[string]string{"namespace": namespace})
	if err != nil {
		return err
	}
	var list []vo.PolicyVo
	if err = json.Unmarshal(data, &list); err != nil {
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

// ListTokens prints all enrollment tokens, optionally filtered by namespace.
func (c *Client) ListTokens(namespace string) error {
	data, err := c.call("token.list", map[string]string{"namespace": namespace})
	if err != nil {
		return err
	}
	var list []*models.Token
	if err = json.Unmarshal(data, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no tokens found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TOKEN\tNAMESPACE\tLIMIT\tEXPIRY") //nolint:errcheck
	for _, t := range list {
		expiry := t.Expiry
		if expiry == "" {
			expiry = "never"
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
func (c *Client) RemoveToken(token string) error {
	_, err := c.call("token.remove", map[string]string{"token": token})
	if err != nil {
		return err
	}
	fmt.Printf("token %q revoked\n", token)
	return nil
}
