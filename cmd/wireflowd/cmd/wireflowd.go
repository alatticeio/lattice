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

package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"wireflow/internal/config"
	"wireflow/internal/controller"
	"wireflow/internal/db"
	internalnats "wireflow/internal/nats"
	"wireflow/management"

	"golang.org/x/sync/errgroup"
)

const embeddedNATSPort = 4222

func runWireflowd(flags *config.Config) error {
	// 1. 创建全局上下文，响应系统信号（Ctrl+C）
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	fmt.Println("Wireflowd is starting all-in-one mode...")

	// 2. 启动嵌入式 NATS (基础设施)；natsReady 在 NATS 就绪后关闭
	natsReady := make(chan struct{})
	g.Go(func() error {
		fmt.Println("Starting embedded NATS server...")
		return internalnats.RunEmbedded(ctx, embeddedNATSPort, natsReady)
	})

	// 3. 初始化数据库（SQLite 开源默认，MariaDB 生产环境）
	fmt.Println("Initializing storage...")
	_, err := db.NewStore(flags)
	if err != nil {
		return fmt.Errorf("failed to init db: %w", err)
	}

	// 4. 启动 K8s 控制器和业务管理器 (逻辑层)
	g.Go(func() error {
		fmt.Println("Starting Wireflow Controllers...")
		return controller.Start(flags)
	})

	// 5. 等待 NATS 就绪后再启动 management
	g.Go(func() error {
		select {
		case <-natsReady:
		case <-ctx.Done():
			return ctx.Err()
		}
		fmt.Println("Starting Wireflow Manager...")
		// all-in-one 模式下，若用户未配置 signaling-url，则使用内嵌 NATS 地址
		if flags.SignalingURL == "" {
			flags.SignalingURL = fmt.Sprintf("nats://localhost:%d", embeddedNATSPort)
		}
		return management.Start(flags)
	})

	// 5. 等待所有组件运行，或者其中一个报错退出
	fmt.Println("All systems go! Wireflowd is ready.")

	if err := g.Wait(); err != nil {
		return fmt.Errorf("wireflowd stopped with error: %w", err)
	}

	fmt.Println("Wireflowd stopped gracefully.")
	return nil
}
