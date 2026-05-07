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

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alatticeio/lattice/internal/server/llm"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

// fakeAIService is a test double for service.AIService.
type fakeAIService struct{}

func (f *fakeAIService) Chat(_ context.Context, _ *service.ChatRequest, _ service.StreamWriter) error {
	return nil
}
func (f *fakeAIService) Audit(_ context.Context, _ string) (*service.AuditReport, error) {
	return &service.AuditReport{Score: 100}, nil
}
func (f *fakeAIService) Debug(_ context.Context, _ *service.DebugRequest, _ service.StreamWriter) error {
	return nil
}
func (f *fakeAIService) ListTools(_ string) []llm.Tool {
	return []llm.Tool{
		{Name: "list_peers", Description: "列出 Peers", InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "create_policy", Description: "创建策略", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
}
func (f *fakeAIService) ExecuteTool(_ context.Context, _, name string, _ json.RawMessage) (string, error) {
	return "result from " + name, nil
}

func setupAITestRouter(svc service.AIService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/ai/tools", handleAIListToolsForTest(svc))
	r.POST("/api/v1/ai/tools/call", handleAIToolCallForTest(svc))
	return r
}

// handleAIListToolsForTest is a thin wrapper that avoids full server setup.
func handleAIListToolsForTest(svc service.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ns := c.Query("namespace")
		if ns == "" {
			ns = "default"
		}
		tools := svc.ListTools(ns)
		c.JSON(http.StatusOK, gin.H{"data": tools})
	}
}

func handleAIToolCallForTest(svc service.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Namespace string          `json:"namespace"`
			Tool      string          `json:"tool"`
			Input     json.RawMessage `json:"input"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Namespace == "" {
			req.Namespace = "default"
		}
		if req.Input == nil {
			req.Input = json.RawMessage(`{}`)
		}
		result, err := svc.ExecuteTool(c.Request.Context(), req.Namespace, req.Tool, req.Input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"result": result}})
	}
}

func TestHandleAIListTools(t *testing.T) {
	g := NewWithT(t)
	r := setupAITestRouter(&fakeAIService{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/tools?namespace=default", nil)
	r.ServeHTTP(w, req)

	g.Expect(w.Code).To(Equal(http.StatusOK))
	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&body)
	g.Expect(body.Data).To(HaveLen(2))
	g.Expect(body.Data[0]["name"]).To(Equal("list_peers"))
	g.Expect(body.Data[1]["name"]).To(Equal("create_policy"))
}

func TestHandleAIToolCall(t *testing.T) {
	g := NewWithT(t)
	r := setupAITestRouter(&fakeAIService{})

	body, _ := json.Marshal(map[string]interface{}{
		"namespace": "test-ns",
		"tool":      "list_peers",
		"input":     map[string]interface{}{},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	g.Expect(w.Code).To(Equal(http.StatusOK))
	var resp struct {
		Data struct {
			Result string `json:"result"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	g.Expect(resp.Data.Result).To(Equal("result from list_peers"))
}
