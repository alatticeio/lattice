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

// Package mcpproxy implements an HTTP proxy that intercepts MCP tool calls,
// enforces AgentPolicy, and writes structured audit events.
package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyConfig holds the MCPServer and AgentPolicy lists fetched from the server.
type PolicyConfig struct {
	MCPServers    []v1alpha1.MCPServer   `json:"mcpServers"`
	AgentPolicies []v1alpha1.AgentPolicy `json:"agentPolicies"`
}

// PolicyCache fetches and caches MCPServer + AgentPolicy config from the Lattice
// management server, refreshing every refreshInterval. Thread-safe.
type PolicyCache struct {
	serverURL   string
	agentJWT    string
	namespace   string
	refreshRate time.Duration

	mu  sync.RWMutex
	cfg *PolicyConfig
}

// NewPolicyCache creates a PolicyCache. Call Start to begin background refresh.
func NewPolicyCache(serverURL, agentJWT, namespace string) *PolicyCache {
	return &PolicyCache{
		serverURL:   serverURL,
		agentJWT:    agentJWT,
		namespace:   namespace,
		refreshRate: 15 * time.Second,
	}
}

// Start fetches the initial config (blocking) then launches a background refresh loop.
// Returns an error if the initial fetch fails.
func (c *PolicyCache) Start(ctx context.Context) error {
	if err := c.fetch(ctx); err != nil {
		return fmt.Errorf("mcpproxy: initial policy fetch: %w", err)
	}
	go c.loop(ctx)
	return nil
}

// Get returns the current cached config. Returns empty config if not yet loaded.
func (c *PolicyCache) Get() PolicyConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cfg == nil {
		return PolicyConfig{}
	}
	return *c.cfg
}

// IsToolAllowed checks whether the given (agentName, mcpServerName, toolName)
// combination is permitted by at least one AgentPolicy in the cache.
// If no policy has DefaultDeny, all tools are allowed (audit-only mode).
func (c *PolicyCache) IsToolAllowed(agentName, mcpServerName, toolName string) bool {
	cfg := c.Get()

	for _, policy := range cfg.AgentPolicies {
		if !selectorMatchesAgent(policy.Spec.AgentSelector, agentName) {
			continue
		}
		if !policy.Spec.DefaultDeny {
			// This policy is allow-all; don't block.
			return true
		}
		// DefaultDeny: check allowedTools.
		for _, perm := range policy.Spec.AllowedTools {
			if perm.MCPServer != mcpServerName {
				continue
			}
			for _, t := range perm.Tools {
				if t == "*" || t == toolName {
					return true
				}
			}
		}
	}

	// No policy matched with DefaultDeny — allow (no policy = allow all).
	for _, policy := range cfg.AgentPolicies {
		if selectorMatchesAgent(policy.Spec.AgentSelector, agentName) && policy.Spec.DefaultDeny {
			return false // at least one DefaultDeny policy matched and denied
		}
	}
	return true
}

// MCPServerForHost returns the MCPServer whose endpoint host matches the given host,
// or nil if no match. Used for external MCPs.
func (c *PolicyCache) MCPServerForHost(host string) *v1alpha1.MCPServer {
	cfg := c.Get()
	for i := range cfg.MCPServers {
		srv := &cfg.MCPServers[i]
		u, err := url.Parse(srv.Spec.Endpoint)
		if err != nil {
			continue
		}
		if u.Hostname() == host {
			return srv
		}
	}
	return nil
}

// MCPServerForIP returns the MCPServer whose overlay IP matches, or nil.
// Used for internal MCPs.
func (c *PolicyCache) MCPServerForIP(ip string) *v1alpha1.MCPServer {
	cfg := c.Get()
	for i := range cfg.MCPServers {
		srv := &cfg.MCPServers[i]
		if srv.Status.PeerAddress == ip {
			return srv
		}
	}
	return nil
}

func (c *PolicyCache) loop(ctx context.Context) {
	ticker := time.NewTicker(c.refreshRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.fetch(ctx) // errors are non-fatal; cached config remains valid
		}
	}
}

func (c *PolicyCache) fetch(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/api/v1/agent/mcp-config?namespace=%s", c.serverURL, c.namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.agentJWT)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Data PolicyConfig `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode mcp config: %w", err)
	}

	c.mu.Lock()
	c.cfg = &envelope.Data
	c.mu.Unlock()
	return nil
}

// selectorMatchesAgent returns true if the label selector matches this agent.
// MVP: all policies apply to all agents (filtering by selector in Phase C).
func selectorMatchesAgent(_ metav1.LabelSelector, _ string) bool {
	return true
}
