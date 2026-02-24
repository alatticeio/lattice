package server

import (
	"github.com/gin-gonic/gin"
)

// nolint:all
func (s *Server) dashboardRouter() {
	dashApi := s.Group("/api/v1/dashboard")
	{
		dashApi.GET("/overview", s.dashboardOverview())
	}
}

// nolint:all
func (s *Server) dashboardOverview() gin.HandlerFunc {
	return func(c *gin.Context) {
		//var wg sync.WaitGroup
		//var overview vo.DashboardVo
		//
		//wg.Add(3)
		//// 1. 统计在线节点
		//go func() {
		//	defer wg.Done()
		//	// 模拟数据库查询: db.Where("status = ?", "online").Count(&overview.OnlineNodes)
		//	overview.OnlineNodes = 12
		//}()
		//
		//// 2. 统计策略总数
		//go func() {
		//	defer wg.Done()
		//	overview.PoliciesCount = 8
		//}()
		//
		//// 3. 统计活跃隧道
		//go func() {
		//	defer wg.Done()
		//	overview.ActiveTunnels = 23
		//}()
		//
		//wg.Wait() // 等待所有查询完成
		//overview.SystemHealth = 99.98
		//overview.AlertsCount = 0
		//
		//c.JSON(200, overview)
	}
}
