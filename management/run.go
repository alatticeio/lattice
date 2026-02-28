// Copyright 2025 The Wireflow Authors, Inc.
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

package management

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management/server"

	"golang.org/x/sync/errgroup"
)

func Start(flags *config.Flags) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := log.GetLogger("management")

	cfg := config.InitConfig(flags.ENV)

	// 1. 初始化服务实例
	hs, err := server.NewServer(&server.ServerConfig{
		Cfg: cfg,
	})
	if err != nil {
		return err
	}

	// 2. 创建 errgroup
	g, ctx := errgroup.WithContext(ctx)

	// 任务 A: 启动 Manager (控制器逻辑)
	g.Go(func() error {
		logger.Info("Starting Manager...")
		// hs.Start 内部应该封装了 mgr.Start(ctx)
		return hs.Start(ctx)
	})

	// 任务 B: 等待缓存同步并启动 HTTP Server
	g.Go(func() error {
		logger.Info("Waiting for cache sync...")
		// 关键：确保在启动 Web 服务前，缓存已经同步完成
		if !hs.GetManager().GetCache().WaitForCacheSync(ctx) {
			return fmt.Errorf("failed to wait for cache sync")
		}
		logger.Info("Cache synced, starting API Server...")

		srv := &http.Server{
			Addr:    ":8080",
			Handler: hs, // 你的 gin.Engine
		}

		// 独立协程负责监听 ctx.Done 并执行 Shutdown
		go func() {
			<-ctx.Done()
			logger.Info("Shutting down API Server...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()

		// 使用 ListenAndServe 而非 hs.Run，因为优雅退出逻辑已在上面 go func 中处理
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// 3. 阻塞等待所有任务
	if err := g.Wait(); err != nil {
		logger.Error("System exited with error:", err)
		return err
	}

	return nil
}
