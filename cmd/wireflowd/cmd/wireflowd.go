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
	"wireflow/internal/nats"
	"wireflow/management"

	"golang.org/x/sync/errgroup"
)

func runWireflowd(flags *config.Config) error {
	// 1. 创建全局上下文，响应系统信号（Ctrl+C）
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	fmt.Println("🌊 Wireflowd is starting all-in-one mode...")

	// 2. 启动嵌入式 NATS (基础设施)
	g.Go(func() error {
		fmt.Println("  [1/3] 🔌 Starting embedded NATS server...")
		return nats.RunEmbedded(ctx, 4222)
	})

	// 3. 初始化数据库（SQLite 开源默认，MariaDB 生产环境）
	fmt.Println("  [2/3] 📂 Initializing storage...")
	_, err := db.NewStore(flags)
	if err != nil {
		return fmt.Errorf("failed to init db: %w", err)
	}

	// 4. 启动 K8s 控制器和业务管理器 (逻辑层)
	g.Go(func() error {
		fmt.Println("  [3/3] 🧠 Starting Wireflow Controllers...")
		// 传入数据库实例和 NATS 连接地址
		return controller.Start(flags)
	})

	g.Go(func() error {
		fmt.Println("Starting Wireflow Manager...")
		return management.Start(flags)
	})

	// 5. 等待所有组件运行，或者其中一个报错退出
	fmt.Println("All systems go! Wireflowd is ready.")

	if err := g.Wait(); err != nil {
		return fmt.Errorf("wireflowd stopped with error: %w", err)
	}

	fmt.Println(" Wireflowd stopped gracefully.")
	return nil
}
