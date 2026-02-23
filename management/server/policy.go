package server

import (
	"wireflow/management/dto"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) listPolicies(c *gin.Context) {
	var req dto.PageRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
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

func (s *Server) updatePolicy(c *gin.Context) {
	var req dto.PeerDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	vo, err := s.peerController.UpdatePeer(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	resp.OK(c, vo)
}

func (s *Server) createOrUpdatePolicy(c *gin.Context) {
	var req dto.PolicyDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	vo, err := s.policyController.CreateOrUpdatePolicy(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	resp.OK(c, vo)
}

func (s *Server) deletePolicy(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		resp.BadRequest(c, "policy name is required")
		return
	}

	err := s.policyController.DeletePolicy(c.Request.Context(), name)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	resp.OK(c, nil)
}
