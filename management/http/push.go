package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"time"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/manager"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// 推送请求结构体
type PushRequest struct {
	Content string `json:"content,omitempty"`
	URL     string `json:"url,omitempty"`
}

// 推送响应结构体
type PushResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

type HttpServer struct {
	wt domain.IWatchManager
}

func NewPush() {
	ctx := context.Background()
	logger := klog.FromContext(ctx)
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 加载模板文件
	router.LoadHTMLGlob("./management/templates/*")

	// 静态文件服务
	router.Static("/static", "./management/static")

	// 首页 - 推送页面
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "推送管理平台",
		})
	})

	s := &HttpServer{
		wt: manager.NewWatchManager(),
	}

	// 推送 API 接口
	router.POST("/api/push", s.pushHandler)

	// 批量推送接口
	//router.POST("/api/push/batch", batchPushHandler)

	// 获取推送历史
	router.GET("/api/push/history", pushHistoryHandler)

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "推送服务运行正常",
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	logger.Info("推送服务启动在 http://localhost:8081")
	logger.Info("访问 http://localhost:8081 使用推送功能")

	if err := router.Run(":8082"); err != nil {
		panic(fmt.Sprintf("服务启动失败: %v", err))
	}
}

func getCurrentDir() string {
	// 获取当前文件的完整路径
	_, file, _, _ := runtime.Caller(1)
	// 返回当前文件所在的目录
	return filepath.Dir(file)
}

// 推送处理函数
func (s *HttpServer) pushHandler(c *gin.Context) {
	var req PushRequest

	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PushResponse{
			Success: false,
			Message: fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 验证内容长度
	if len(req.Content) > 5000 {
		c.JSON(http.StatusBadRequest, PushResponse{
			Success: false,
			Message: "推送内容过长，请控制在5000字符以内",
		})
		return
	}

	// 执行推送
	result, err := s.sendPush(req.URL, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, PushResponse{
			Success: false,
			Message: fmt.Sprintf("推送失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, PushResponse{
		Success: true,
		Message: "推送成功",
		Data:    result,
	})
}

//
//// 批量推送处理
//func batchPushHandler(c *gin.Context) {
//	var requests struct {
//		Pushes []PushRequest `json:"pushes" binding:"required,min=1,max=10"`
//	}
//
//	if err := c.ShouldBindJSON(&requests); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"success": false,
//			"message": fmt.Sprintf("请求参数错误: %v", err),
//		})
//		return
//	}
//
//	results := make([]gin.H, 0, len(requests.Pushes))
//
//	for i, req := range requests.Pushes {
//		result, err := sendPush(req.URL, req.Content)
//		status := "success"
//		if err != nil {
//			status = "failed"
//		}
//
//		results = append(results, gin.H{
//			"index":  i + 1,
//			"url":    req.URL,
//			"status": status,
//			"result": result,
//			"error":  fmt.Sprintf("%v", err),
//		})
//	}
//
//	c.JSON(http.StatusOK, gin.H{
//		"success": true,
//		"message": "批量推送完成",
//		"results": results,
//	})
//}

// 推送历史接口
func pushHistoryHandler(c *gin.Context) {
	// 这里可以连接数据库获取历史记录
	// 暂时返回模拟数据
	history := []gin.H{
		{
			"time":    time.Now().Add(-5 * time.Minute).Format("15:04:05"),
			"url":     "http://example.com/webhook",
			"content": "测试推送消息1",
			"status":  "success",
		},
		{
			"time":    time.Now().Add(-10 * time.Minute).Format("15:04:05"),
			"url":     "http://api.example.com/notify",
			"content": "测试推送消息2",
			"status":  "failed",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
	})
}

// 实际发送推送的函数
func (s *HttpServer) sendPush(clientId, content string) (string, error) {
	var msg domain.Message
	// 创建请求数据
	if err := json.Unmarshal([]byte(content), &msg); err != nil {
		return "", fmt.Errorf("json unmarshal failed: %v", err)
	}

	s.wt.Send(clientId, &msg)

	return "success", nil

}
