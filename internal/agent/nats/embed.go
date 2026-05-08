package nats

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// RunEmbedded starts an embedded NATS Server (with JetStream) within the current process.
//
// ready is closed when the Server is ready, allowing callers to detect startup completion; pass nil to ignore.
// The function blocks until ctx is cancelled, then performs a graceful shutdown.
func RunEmbedded(ctx context.Context, port int, ready chan<- struct{}) error {
	storeDir := os.Getenv("NATS_STORE_DIR")
	if storeDir == "" {
		storeDir = "data/nats-jetstream"
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("create nats store dir: %w", err)
	}

	opts := &server.Options{
		Host:   "0.0.0.0",
		Port:   port,
		NoSigs: true, // lifecycle controlled by external ctx, prevent NATS from capturing signals
		NoLog:  true, // suppress NATS internal logs in embedded mode to avoid noise

		// JetStream persistence
		JetStream: true,
		StoreDir:  storeDir,

		// Embedded mode does not enable authentication; clients connect anonymously.
		// If authentication is needed, inject via NATS_USERNAME / NATS_PASSWORD environment variables,
		// and configure server.Options.Username / Password here.
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
