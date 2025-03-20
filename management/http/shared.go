package http

import (
	"github.com/gin-gonic/gin"
	"strconv"
)

func (s *Server) RegisterSharedRoutes() {
	userGroup := s.RouterGroup.Group(PREFIX + "/shared")
	userGroup.DELETE("/invite/:inviteId/label/:labelId", s.deleteSharedLabel())
	userGroup.DELETE("/invite/:inviteId/group/:groupId", s.deleteSharedGroup())
	userGroup.DELETE("/invite/:inviteId/node/:nodeId", s.deleteSharedNode())
	userGroup.DELETE("/invite/:inviteId/policy/:policyId", s.deleteSharedPolicy())

}

func (s *Server) deleteSharedLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		lid := c.Param("labelId")
		labelId, err := strconv.Atoi(lid)
		if err != nil {
			WriteError(c.JSON, "invalid label id")
			return
		}
		err = s.sharedController.DeleteSharedLabel(c, uint(inviteId), uint(labelId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteSharedGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		gid := c.Param("groupId")
		groupId, err := strconv.Atoi(gid)
		if err != nil {
			WriteError(c.JSON, "invalid group id")
			return
		}
		err = s.sharedController.DeleteSharedGroup(c, uint(inviteId), uint(groupId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteSharedNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		nid := c.Param("nodeId")
		nodeId, err := strconv.Atoi(nid)
		if err != nil {
			WriteError(c.JSON, "invalid node id")
			return
		}
		err = s.sharedController.DeleteSharedNode(c, uint(inviteId), uint(nodeId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteSharedPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		pid := c.Param("policyId")
		policyId, err := strconv.Atoi(pid)
		if err != nil {
			WriteError(c.JSON, "invalid policy id")
			return
		}
		err = s.sharedController.DeleteSharedPolicy(c, uint(inviteId), uint(policyId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}
