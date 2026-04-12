package nats

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// RunEmbedded 在当前进程内启动一个嵌入式 NATS Server（含 JetStream）。
//
// ready 在 Server 就绪后关闭，调用方可借此感知启动完成；传 nil 则忽略。
// 函数会阻塞直到 ctx 被取消，随后执行优雅关闭。
func RunEmbedded(ctx context.Context, port int, ready chan<- struct{}) error {
	storeDir := os.Getenv("NATS_STORE_DIR")
	if storeDir == "" {
		storeDir = "data/nats-jetstream"
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("create nats store dir: %w", err)
	}

	opts := &server.Options{
		Host:     "0.0.0.0",
		Port:     port,
		NoSigs:   true, // 由外部 ctx 控制生命周期，禁止 NATS 自行捕获信号
		NoLog:    true, // 嵌入模式下屏蔽 NATS 内部日志，避免刷屏

		// JetStream 持久化
		JetStream: true,
		StoreDir:  storeDir,

		// 嵌入模式不启用认证，客户端匿名连接即可。
		// 若需认证，通过 NATS_USERNAME / NATS_PASSWORD 环境变量注入，
		// 并在此处配置 server.Options.Username / Password。
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("create nats server: %w", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("nats server did not become ready within 10s")
	}

	log.Printf("embedded NATS running at nats://0.0.0.0:%d  store=%s", port, storeDir)

	if ready != nil {
		close(ready)
	}

	<-ctx.Done()

	log.Println("shutting down embedded NATS...")
	ns.Shutdown()
	ns.WaitForShutdown()
	return nil
}
