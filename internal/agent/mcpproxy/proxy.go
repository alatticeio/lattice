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

package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Proxy is an HTTP proxy that intercepts MCP JSON-RPC calls, enforces AgentPolicy,
// and writes audit events. Non-MCP traffic is forwarded transparently.
type Proxy struct {
	agentName string
	cache     *PolicyCache
	audit     *AuditWriter
	server    *http.Server
}

// NewProxy creates a Proxy. addr is the listen address (e.g. "127.0.0.1:15002").
func NewProxy(agentName, addr string, cache *PolicyCache, audit *AuditWriter) *Proxy {
	p := &Proxy{
		agentName: agentName,
		cache:     cache,
		audit:     audit,
	}
	p.server = &http.Server{
		Addr:    addr,
		Handler: p,
	}
	return p
}

// Addr returns the listen address. Call after Start.
func (p *Proxy) Addr() string { return p.server.Addr }

// Start begins listening. Returns once the listener is bound; handler runs in background.
func (p *Proxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.server.Addr)
	if err != nil {
		return fmt.Errorf("mcpproxy: listen: %w", err)
	}
	// Update addr to the actual bound address (port 0 → OS-assigned port).
	p.server.Addr = ln.Addr().String()
	go func() {
		<-ctx.Done()
		_ = p.server.Shutdown(context.Background())
	}()
	go func() { _ = p.server.Serve(ln) }()
	return nil
}

// ServeHTTP implements http.Handler. Routes CONNECT tunnels and regular requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

// handleHTTP handles plain HTTP proxy requests.
// If the target is a known MCPServer endpoint, parse the JSON-RPC body and enforce policy.
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	targetHost := r.URL.Hostname()

	// Check if this request targets a known MCPServer.
	mcpSrv := p.cache.MCPServerForHost(targetHost)

	if mcpSrv != nil && r.Method == http.MethodPost {
		// Read body for inspection.
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadGateway)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		toolName, params := extractMCPTool(body)
		if toolName != "" {
			paramSummary := summarizeParams(params)
			if !p.cache.IsToolAllowed(p.agentName, mcpSrv.Name, toolName) {
				p.audit.Write(MCPAuditEvent{
					AgentName:    p.agentName,
					MCPServer:    mcpSrv.Name,
					Tool:         toolName,
					ParamSummary: paramSummary,
					Verdict:      verdictDeny,
					DenyReason:   "AgentPolicy denied",
				})
				http.Error(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Lattice AgentPolicy: tool not allowed"}}`,
					http.StatusForbidden)
				return
			}
			p.audit.Write(MCPAuditEvent{
				AgentName:    p.agentName,
				MCPServer:    mcpSrv.Name,
				Tool:         toolName,
				ParamSummary: paramSummary,
				Verdict:      verdictAllow,
			})
		}
	}

	// Forward the request to the real target.
	targetURL, err := url.Parse(r.RequestURI)
	if err != nil || targetURL.Host == "" {
		targetURL = &url.URL{
			Scheme: "http",
			Host:   r.Host,
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

// handleConnect handles HTTPS CONNECT tunnels.
// For HTTPS MCPs (external platform MCPs), we cannot inspect the payload (TLS).
// We pass through transparently and log at the connection level only.
func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Dial the target.
	conn, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "failed to connect", http.StatusBadGateway)
		return
	}
	defer conn.Close() //nolint:errcheck

	// Hijack the client connection.
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close() //nolint:errcheck

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional copy.
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(conn, clientConn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientConn, conn); done <- struct{}{} }()
	<-done
}

// mcpRequest is the minimal JSON-RPC 2.0 structure for MCP tool calls.
type mcpRequest struct {
	Method string `json:"method"`
	Params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
}

// extractMCPTool parses a JSON-RPC body and returns the tool name and arguments
// if the method is "tools/call". Returns ("", nil) for non-tool-call requests.
func extractMCPTool(body []byte) (toolName string, params map[string]interface{}) {
	var req mcpRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", nil
	}
	if req.Method != "tools/call" {
		return "", nil
	}
	return req.Params.Name, req.Params.Arguments
}
