package cmd

import (
	"context"
	"fmt"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/pkg/cmd"
)

func runVersion() error {
	if config.GlobalConfig.SignalUrl == "" {
		config.GlobalConfig.SignalUrl = fmt.Sprintf("nats://%s:%d", infra.SignalingDomain, infra.DefaultSignalingPort)
		config.WriteConfig("siganl-url", config.GlobalConfig.SignalUrl)
	}
	client, err := cmd.NewClient(config.GlobalConfig.SignalUrl)
	if err != nil {
		return err
	}

	return client.Info(context.Background())
}
