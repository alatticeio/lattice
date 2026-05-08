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

package mcp_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alatticeio/lattice/internal/mcp"
	. "github.com/onsi/gomega"
)

func TestMCPServer_Initialize(t *testing.T) {
	g := NewWithT(t)
	in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}` + "\n")
	out := &bytes.Buffer{}

	srv := mcp.NewServer("https://lattice.test", "token-xxx", mcp.ServerOptions{})
	err := srv.HandleOnce(in, out)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	_ = json.NewDecoder(bufio.NewReader(out)).Decode(&resp)
	g.Expect(resp.JSONRPC).To(Equal("2.0"))
	g.Expect(resp.ID).To(Equal(1))
	g.Expect(resp.Result.ServerInfo.Name).To(Equal("lattice"))
	g.Expect(resp.Result.ProtocolVersion).To(Equal("2024-11-05"))
}

func TestMCPServer_ToolsList(t *testing.T) {
	g := NewWithT(t)
	mockTools := []map[string]interface{}{
		{"name": "list_peers", "description": "List Peers", "inputSchema": map[string]interface{}{"type": "object"}},
	}
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ai/tools" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": mockTools})
		}
	}))
	defer mockServer.Close()

	in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	out := &bytes.Buffer{}
	srv := mcp.NewServer(mockServer.URL, "token-xxx", mcp.ServerOptions{WorkspaceID: "ws-test"})
	err := srv.HandleOnce(in, out)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
	}
	_ = json.NewDecoder(out).Decode(&resp)
	g.Expect(resp.Result.Tools).To(HaveLen(1))
	g.Expect(resp.Result.Tools[0]["name"]).To(Equal("list_peers"))
}

func TestMCPServer_ToolsCall(t *testing.T) {
	g := NewWithT(t)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ai/tools/call" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"result": "Total 2 Peers"},
			})
		}
	}))
	defer mockServer.Close()

	in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_peers","arguments":{}}}` + "\n")
	out := &bytes.Buffer{}
	srv := mcp.NewServer(mockServer.URL, "token-xxx", mcp.ServerOptions{WorkspaceID: "ws-test"})
	err := srv.HandleOnce(in, out)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	_ = json.NewDecoder(out).Decode(&resp)
	g.Expect(resp.Result.IsError).To(BeFalse())
	g.Expect(resp.Result.Content).To(HaveLen(1))
	g.Expect(resp.Result.Content[0].Text).To(Equal("Total 2 Peers"))
}

func TestMCPServer_UnknownMethod(t *testing.T) {
	g := NewWithT(t)
	in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":4,"method":"unknown/method","params":{}}` + "\n")
	out := &bytes.Buffer{}

	srv := mcp.NewServer("https://lattice.test", "token", mcp.ServerOptions{})
	err := srv.HandleOnce(in, out)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(out).Decode(&resp)
	g.Expect(resp.Error.Code).To(Equal(-32601))
	g.Expect(resp.Error.Message).To(ContainSubstring("method not found"))
}
