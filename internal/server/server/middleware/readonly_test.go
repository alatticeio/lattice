// Copyright 2024 The Lattice Authors.
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

package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

func newReadOnlyRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ReadOnly())
	router.POST("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.PUT("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.DELETE("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return router
}

func TestReadOnly_BlocksPOST(t *testing.T) {
	router := newReadOnlyRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/test", nil)
	router.ServeHTTP(w, req)

	var r resp.Response
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", w.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected code 403 for POST, got %d", r.Code)
	}
}

func TestReadOnly_AllowsGET(t *testing.T) {
	router := newReadOnlyRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for GET, got %d", w.Code)
	}
}

func TestReadOnly_BlocksPUT(t *testing.T) {
	router := newReadOnlyRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/test", nil)
	router.ServeHTTP(w, req)

	var r resp.Response
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", w.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected code 403 for PUT, got %d", r.Code)
	}
}

func TestReadOnly_BlocksDELETE(t *testing.T) {
	router := newReadOnlyRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/test", nil)
	router.ServeHTTP(w, req)

	var r resp.Response
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", w.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected code 403 for DELETE, got %d", r.Code)
	}
}
