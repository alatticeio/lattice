package server

import (
	"wireflow/management/dto"

	"github.com/gin-gonic/gin"
)

func (s *Server) listPolicies(c *gin.Context) {
	var req dto.PageRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		WriteBadRequest(c.JSON, err.Error())
		return
	}

	vo, err := s.policyController.ListPolicy(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	WriteOK(c.JSON, vo)
}

func (s *Server) updatePolicy(c *gin.Context) {
	var req dto.PeerDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		WriteBadRequest(c.JSON, err.Error())
		return
	}

	vo, err := s.peerController.UpdatePeer(c.Request.Context(), &req)
	if err != nil {
		WriteError(c.JSON, err.Error())
		return
	}

	c.JSON(200, vo)
}

func (s *Server) createPolicy(c *gin.Context) {
	var req dto.PeerDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		WriteBadRequest(c.JSON, err.Error())
		return
	}

	vo, err := s.peerController.UpdatePeer(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, vo)
}

func (s *Server) deletePolicy(c *gin.Context) {
	var req dto.PeerDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		WriteBadRequest(c.JSON, err.Error())
		return
	}

	vo, err := s.peerController.UpdatePeer(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, vo)
}
