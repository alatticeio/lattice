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

	// 2. Create errgroup
	g, ctx := errgroup.WithContext(ctx)
	// GlobalConfig is populated in PersistentPreRunE by config.GetManager().Load(cmd).
	hs, err := server.NewServer(ctx, &server.ServerConfig{
		Cfg: config.GlobalConfig,
	})
	if err != nil {
		return err
	}

	// Task A: Start Manager (controller logic)
	g.Go(func() error {
		logger.Info("management server starting")
		// hs.Start should internally encapsulate mgr.Start(ctx)
		return hs.Start(ctx)
	})

	// Task B: Wait for cache sync (if K8s is available) then start HTTP Server
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
			Handler: hs, // Your gin.Engine
		}

		// Separate goroutine to listen for ctx.Done and execute Shutdown
		go func() {
			<-ctx.Done()
			logger.Info("API server shutting down")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()

		// Use ListenAndServe instead of hs.Run because graceful shutdown is already handled in the go func above
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// 3. Block and wait for all tasks
	if err := g.Wait(); err != nil {
		logger.Error("management server exited with error", err)
		return err
	}

	return nil
}
