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

package server

import (
	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
)

// mcpServerRouter registers MCPServer CRUD endpoints.
func (s *Server) mcpServerRouter() {
	g := s.Group("/api/v1/mcp-servers")
	g.Use(middleware.AuthMiddleware(s.revocationList))
	{
		g.GET("", s.handleListMCPServers())
		g.POST("", s.handleCreateMCPServer())
		g.GET("/:name", s.handleGetMCPServer())
		g.PUT("/:name", s.handleUpdateMCPServer())
		g.DELETE("/:name", s.handleDeleteMCPServer())
		g.GET("/:name/tools", s.handleListMCPServerTools())
	}
}

func (s *Server) handleListMCPServers() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		items, err := s.mcpServerSvc.List(c.Request.Context(), ns)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, items)
	}
}

func (s *Server) handleCreateMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Name string                 `json:"name" binding:"required"`
			Spec v1alpha1.MCPServerSpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.mcpServerSvc.Create(c.Request.Context(), ns, req.Spec, req.Name)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleGetMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		obj, err := s.mcpServerSvc.Get(c.Request.Context(), ns, c.Param("name"))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleUpdateMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		var req struct {
			Spec v1alpha1.MCPServerSpec `json:"spec" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request: "+err.Error())
			return
		}
		obj, err := s.mcpServerSvc.Update(c.Request.Context(), ns, c.Param("name"), req.Spec)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj)
	}
}

func (s *Server) handleDeleteMCPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		if err := s.mcpServerSvc.Delete(c.Request.Context(), ns, c.Param("name")); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func (s *Server) handleListMCPServerTools() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mcpServerSvc == nil {
			resp.PaymentRequired(c, "MCP server management requires K8s")
			return
		}
		ns := c.Query("namespace")
		if ns == "" {
			ns = c.GetString("workspace_id")
		}
		obj, err := s.mcpServerSvc.Get(c.Request.Context(), ns, c.Param("name"))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, obj.Spec.Tools)
	}
}
