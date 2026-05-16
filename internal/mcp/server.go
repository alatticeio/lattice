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

// Package mcp implements an MCP (Model Context Protocol) stdio server that
// proxies tool calls to a running Lattice API server.
//
// The server speaks JSON-RPC 2.0 over stdin/stdout and forwards tool calls
// to the Lattice /api/v1/ai/tools/* endpoints.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// ServerOptions configures the MCP server.
type ServerOptions struct {
	// WorkspaceID scopes all tool calls to a specific Lattice workspace.
	WorkspaceID string
	// Version is the server version string (default: "0.1.0").
	Version string
}

// Server is an MCP stdio server that proxies tool calls to a Lattice API server.
type Server struct {
	latticeURL  string
	authToken   string
	workspaceID string
	version     string
	httpClient  *http.Client
}

// NewServer creates a new MCP server.
//   - latticeURL: base URL of the running latticed instance (e.g. "https://lattice.company.com")
//   - authToken: JWT token for Lattice API authentication
func NewServer(latticeURL, authToken string, opts ServerOptions) *Server {
	version := opts.Version
	if version == "" {
		version = "0.1.0"
	}
	return &Server{
		latticeURL:  latticeURL,
		authToken:   authToken,
		workspaceID: opts.WorkspaceID,
		version:     version,
		httpClient:  &http.Client{},
	}
}

// Run starts the MCP stdio server loop. Reads from os.Stdin, writes to os.Stdout.
// Blocks until stdin is closed or a fatal error occurs.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		response, err := s.handleMessage(line)
		if err != nil {
			response = &Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: err.Error()},
			}
		}
		if response == nil {
			continue // notifications have no response
		}
		out, _ := json.Marshal(response)
		fmt.Fprintf(os.Stdout, "%s\n", out) //nolint:errcheck
	}
	return scanner.Err()
}

// HandleOnce processes a single newline-delimited JSON message from r and writes the response to w.
// Used for testing.
func (s *Server) HandleOnce(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return scanner.Err()
	}
	response, err := s.handleMessage(scanner.Bytes())
	if err != nil {
		response = &Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: err.Error()}}
	}
	if response == nil {
		return nil
	}
	out, _ := json.Marshal(response)
	fmt.Fprintf(w, "%s\n", out) //nolint:errcheck
	return nil
}

func (s *Server) handleMessage(data []byte) (*Response, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil, nil // notification, no response
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: "method not found: " + req.Method},
		}, nil
	}
}

func (s *Server) handleInitialize(req Request) (*Response, error) {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities:    Caps{Tools: &ToolsCap{}},
			ServerInfo:      ServerInfo{Name: "lattice", Version: s.version},
		},
	}, nil
}

func (s *Server) handleToolsList(req Request) (*Response, error) {
	url := fmt.Sprintf("%s/api/v1/ai/tools?workspaceId=%s", s.latticeURL, s.workspaceID)
	body, err := s.get(url)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Data []ToolDef `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse tools response: %w", err)
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: envelope.Data},
	}, nil
}

func (s *Server) handleToolsCall(req Request) (*Response, error) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("parse tool call params: %w", err)
	}
	if params.Arguments == nil {
		params.Arguments = json.RawMessage(`{}`)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"workspaceId": s.workspaceID,
		"tool":        params.Name,
		"input":       params.Arguments,
	})

	body, err := s.post(s.latticeURL+"/api/v1/ai/tools/call", reqBody)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ToolCallResult{
				Content: []ContentBlock{{Type: "text", Text: "error: " + err.Error()}},
				IsError: true,
			},
		}, nil
	}

	var envelope struct {
		Data struct {
			Result string `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse tool call response: %w", err)
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: envelope.Data.Result}},
		},
	}, nil
}

func (s *Server) get(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, buf.String())
	}
	return buf.Bytes(), nil
}

func (s *Server) post(url string, body []byte) ([]byte, error) {
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, buf.String())
	}
	return buf.Bytes(), nil
}
