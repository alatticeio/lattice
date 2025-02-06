package client

import (
	"context"
	"github.com/golang/protobuf/proto"
	pb "linkany/management/grpc/mgt"
	"linkany/pkg/config"
	"testing"
)

func TestNewGrpcClient(t *testing.T) {
	t.Run("TestGrpcClient_List", TestGrpcClient_List)

}

func TestGrpcClient_List(t *testing.T) {
	client, err := NewGrpcClient(&GrpcConfig{Addr: "localhost:50051"})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	requset := &pb.Request{
		AppId: cfg.AppId,
		Token: cfg.Token,
	}

	body, err := proto.Marshal(requset)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	resp, err := client.List(ctx, &pb.ManagementMessage{
		PubKey: "a+BYvXq6/xrvsnKbgORSL6lwFzqtfXV0VnTzwdo+Vnw=",
		Body:   body,
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}
