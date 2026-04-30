package server

import (
	"context"
	"encoding/json"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) listPolicies(c *gin.Context) {
	var req dto.PageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}
	vo, err := s.policyController.ListPolicy(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, vo)
}

func (s *Server) createOrUpdatePolicy(c *gin.Context) {
	var req dto.PolicyDto
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	wsID, _ := c.Request.Context().Value(infra.WorkspaceKey).(string)

	// POST /create
	if c.Request.Method == "POST" {
		// platform_admin applies directly without approval workflow.
		if c.GetString("system_role") == "platform_admin" {
			vo, err := s.policyController.ApplyDirect(c.Request.Context(), wsID, c.GetString("user_id"), c.GetString("username"), &req)
			if err != nil {
				resp.Error(c, err.Error())
				return
			}
			resp.OK(c, vo)
			return
		}

		// Other roles: save as pending + submit workflow for approval.
		policyRec, err := s.policyController.Submit(c.Request.Context(), wsID, c.GetString("user_id"), c.GetString("username"), &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		payload, _ := json.Marshal(map[string]string{"policyId": policyRec.ID})
		v, err := s.workflowController.Submit(c.Request.Context(), service.SubmitWorkflowReq{
			WorkspaceID:      wsID,
			RequestedBy:      c.GetString("user_id"),
			RequestedByName:  c.GetString("username"),
			RequestedByEmail: c.GetString("email"),
			ResourceType:     "policy",
			ResourceName:     req.Name,
			Action:           "create",
			Payload:          string(payload),
		})
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		c.JSON(202, gin.H{"code": 0, "msg": "policy creation submitted for approval", "data": v})
		return
	}

	// PUT /update → apply directly (admin only path).
	vo, err := s.policyController.ApplyDirect(c.Request.Context(), wsID, c.GetString("user_id"), c.GetString("username"), &req)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, vo)
}

// registerPolicyExecutor registers the executor that applies an approved policy.
// Payload is {"policyId": "<id>"} — the DB record ID.
func (s *Server) registerPolicyExecutor() {
	s.workflowService.RegisterExecutor("policy", "create", func(ctx context.Context, payload string) error {
		var p struct {
			PolicyID string `json:"policyId"`
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return err
		}
		return s.policyController.Apply(ctx, p.PolicyID)
	})
}

func (s *Server) deletePolicy(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		resp.BadRequest(c, "policy name is required")
		return
	}
	if err := s.policyController.DeletePolicy(c.Request.Context(), name); err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, nil)
}
