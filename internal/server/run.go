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

package management

import (
	"context"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/server"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

func Start(flags *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := log.GetLogger("management")

	// 2. 创建 errgroup
	g, ctx := errgroup.WithContext(ctx)
	// GlobalConfig 已在 PersistentPreRunE 中由 config.GetManager().Load(cmd) 填充。
	hs, err := server.NewServer(ctx, &server.ServerConfig{
		Cfg: config.GlobalConfig,
	})
	if err != nil {
		return err
	}

	// 任务 A: 启动 Manager (控制器逻辑)
	g.Go(func() error {
		logger.Info("management server starting")
		// hs.Start 内部应该封装了 mgr.Start(ctx)
		return hs.Start(ctx)
	})

	// 任务 B: 等待缓存同步（若 K8s 可用）后启动 HTTP Server
	g.Go(func() error {
		if ch := hs.CacheReady(); ch != nil {
			logger.Info("waiting for informer cache sync")
			select {
			case <-ch:
				logger.Info("cache synced, starting API server")
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			logger.Warn("k8s manager unavailable, starting API server without cache sync")
		}

		srv := &http.Server{
			Addr:    ":8080",
			Handler: hs, // 你的 gin.Engine
		}

		// 独立协程负责监听 ctx.Done 并执行 Shutdown
		go func() {
			<-ctx.Done()
			logger.Info("API server shutting down")
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
		logger.Error("management server exited with error", err)
		return err
	}

	return nil
}
