package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"wireflow/internal/infra"
	"wireflow/management/dto"
	"wireflow/management/nats"
	"wireflow/pkg/version"
)

type Client struct {
	client infra.SignalService
}

func NewClient(signalUrl string) (*Client, error) {
	natsClient, err := nats.NewNatsService(context.Background(), signalUrl)
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
	fmt.Printf("AgentInterface Version: %s\n", clientInfo.Version)
	fmt.Printf("AgentInterface GitCommit: %s\n", clientInfo.GitCommit)
}

func (c *Client) CreateToken(namespace, name, expiry string) error {
	tokenDto := &dto.TokenDto{
		Namespace: namespace,
		Name:      name,
		Expiry:    expiry,
	}

	bs, err := json.Marshal(tokenDto)
	if err != nil {
		return err
	}

	data, err := c.client.Request(context.Background(), "wireflow.signals.service", "createToken", bs)

	if err != nil {
		return err
	}

	fmt.Printf("Token Created: %s\n", string(data))

	return nil
}
