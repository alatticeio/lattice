package client

import (
	"context"
	"github.com/golang/protobuf/proto"
	"linkany/pkg/config"
	"linkany/signaling/grpc/signaling"
	"testing"
)

func TestClient_Register(t *testing.T) {

	client, err := NewClient(&ClientConfig{
		Addr: "console.linkany.io:32132",
	})

	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	publicKey := "123456"
	ctx := context.Background()
	req := &signaling.EncryptMessageRequest{
		SrcPublicKey: publicKey,
		Token:        cfg.Token,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Register(ctx, &signaling.EncryptMessage{
		Body:      body,
		PublicKey: publicKey,
	})

	if err != nil {
		t.Fatal(err)
	}
}
