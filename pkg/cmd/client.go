package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"wireflow/internal/core/infra"
	"wireflow/management/nats"
	"wireflow/pkg/version"
)

type Client struct {
	client infra.SignalService
}

func NewClient(signalUrl string) (*Client, error) {
	natsClient, err := nats.NewNatsService(signalUrl)
	if err != nil {
		return nil, err
	}
	return &Client{client: natsClient}, nil
}

func (c *Client) Info(ctx context.Context) error {

	c.printInfo()
	data, err := c.client.Request(ctx, "wireflow.signals.service", "info", nil)
	if err != nil {
		fmt.Println("Server Version: [Offline/Unknown]")
	} else {
		var serverInfo version.Info
		if err = json.Unmarshal(data, &serverInfo); err != nil {
			fmt.Println("Server Version: [Offline/Unknown]")
			return err
		}
		fmt.Printf("Server Version: %s\n", serverInfo.Version)
		fmt.Printf("Server GitCommit: %s\n", serverInfo.GitCommit)
	}

	return nil
}

func (c *Client) printInfo() {
	clientInfo := version.Get()
	fmt.Printf("Client Version: %s\n", clientInfo.Version)
	fmt.Printf("Client GitCommit: %s\n", clientInfo.GitCommit)
}
