package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"wireflow/internal/infra"
	"wireflow/monitor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	peerManager := infra.NewPeerManager()
	runner := monitor.NewMonitorRunner(peerManager)
	err := runner.Run(ctx)
	if err != nil {
		panic(err)
	}

}
