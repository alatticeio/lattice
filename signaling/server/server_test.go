package server

import (
	client2 "linkany/management/grpc/client"
	"linkany/pkg/config"
	"testing"
)

func TestVerifyToken(t *testing.T) {

	client, err := client2.NewClient(&client2.GrpcConfig{
		Addr: "console.linkany.io:32051",
	})

	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.VerifyToken(cfg.Token)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp.Token == cfg.Token)
}
