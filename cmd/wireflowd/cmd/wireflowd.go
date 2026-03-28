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
