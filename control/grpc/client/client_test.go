package client

import (
	"context"
	pb "linkany/control/grpc/peer"
	"linkany/pkg/config"
	"testing"
)

func TestNewGrpcClient(t *testing.T) {
	client, err := NewGrpcClient(&GrpcConfig{Addr: "localhost:50051"})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	resp, err := client.List(ctx, &pb.Request{
		Username: "linkany",
		AppId:    cfg.AppId,
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp.Peer)
}
