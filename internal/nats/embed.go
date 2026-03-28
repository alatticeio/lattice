package nats

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// RunEmbedded 在当前进程启动一个高度集成的 NATS Server
func RunEmbedded(ctx context.Context, port int) error {
	// 1. 动态确定存储路径 (优先从环境变量读取，否则默认本地 data 目录)
	storeDir := os.Getenv("NATS_STORE_DIR")
	if storeDir == "" {
		storeDir = "data/nats-jetstream"
	}

	// 确保存储目录存在，否则 NATS 启动会报错
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return fmt.Errorf("failed to create nats storage dir: %w", err)
	}

	// 2. 配置 NATS Server 参数
	opts := &server.Options{
		Host:         "0.0.0.0",
		Port:         port,
		NoLog:        false,
		NoSigs:       true, // 在 Go 中嵌入时，建议设为 true，由我们手动控制信号
		MaxPayload:   1024 * 1024,
		PingInterval: 20 * time.Second,

		// 存储相关
		JetStream: true,
		StoreDir:  storeDir,

		// 开源版默认关闭认证，方便本地调试；商业版建议通过 ENV 注入认证配置
		NoAuthUser: "admin",
	}

	// 3. 实例化 Server
	ns, err := server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("could not create nats server: %w", err)
	}

	// 配置 NATS 的内部日志，将其重定向到你的标准日志
	ns.ConfigureLogger()

	// 4. 异步启动
	go ns.Start()

	// 5. 等待 Server 就绪
	if !ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("nats server did not start in time")
	}

	log.Printf("📡 Embedded NATS is running at nats://localhost:%d", port)
	log.Printf("📂 NATS storage path: %s", storeDir)

	// 6. 阻塞等待 Context 结束（优雅退出）
	<-ctx.Done()

	log.Println("🛑 Shutting down embedded NATS server...")
	ns.Shutdown()
	ns.WaitForShutdown()

	return nil
}
