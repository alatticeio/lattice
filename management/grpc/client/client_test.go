package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"linkany/internal"
	"linkany/management/entity"
	pb "linkany/management/grpc/mgt"
	"linkany/management/grpc/server"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"sync"
	"testing"
	"time"
)

var group sync.WaitGroup

func TestNewGrpcClient(t *testing.T) {

	group.Add(2)
	go func() {
		defer group.Done()
		t.Run("TestGrpcClient_List", TestGrpcClient_List)
	}()

	go func() {
		group.Done()
		t.Run("TestGrpcClient_Watch", TestGrpcClient_Watch)
	}()

	group.Wait()

	t.Run("TestGrpcClient_Keepalive", TestGrpcClient_Keepalive)

	t.Run("TestGrpcClient_Register", TestGrpcClient_Register)
}

func TestGrpcClient_List(t *testing.T) {
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051", Logger: log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "grpcclient"))})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	requset := &pb.Request{
		AppId:  cfg.AppId,
		Token:  cfg.Token,
		PubKey: "a+BYvXq6/xrvsnKbgORSL6lwFzqtfXV0VnTzwdo+Vnw=",
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

	var net entity.NetworkMap
	if err = json.Unmarshal(resp.Body, &net); err != nil {
		t.Fatal(err)
	}

	fmt.Println(net)
}

func TestGrpcClient_Watch(t *testing.T) {
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051", Logger: log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "grpcclient"))})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	requset := &pb.Request{
		AppId:  cfg.AppId,
		Token:  cfg.Token,
		PubKey: "a+BYvXq6/xrvsnKbgORSL6lwFzqtfXV0VnTzwdo+Vnw=",
	}

	body, err := proto.Marshal(requset)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	err = client.Watch(ctx, &pb.ManagementMessage{
		PubKey: "a+BYvXq6/xrvsnKbgORSL6lwFzqtfXV0VnTzwdo+Vnw=",
		Body:   body,
	}, func(wm *pb.WatchMessage) error {
		fmt.Println(wm)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

}

func TestGrpcClient_Keepalive(t *testing.T) {
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051", Logger: log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "grpcclient"))})

	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	requset := &pb.Request{
		AppId:  cfg.AppId,
		Token:  cfg.Token,
		PubKey: "a+BYvXq6/xrvsnKbgORSL6lwFzqtfXV0VnTzwdo+Vnw=",
	}

	ctx := context.Background()
	body, err := proto.Marshal(requset)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Keepalive(ctx, &pb.ManagementMessage{
		PubKey: "",
		Body:   body,
	}); err != nil {
		t.Fatal(err)
	}

}

func TestClient_Get(t *testing.T) {
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051", Logger: log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "grpcclient"))})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	requset := &pb.Request{
		AppId:  cfg.AppId,
		Token:  cfg.Token,
		PubKey: "a+BYvXq6/xrvsnKbgORSL6lwFzqtfXV0VnTzwdo+Vnw=",
	}

	ctx := context.Background()
	body, err := proto.Marshal(requset)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get(ctx, &pb.ManagementMessage{Body: body})
	if err != nil {
		t.Fatal(err)
	}

	var peer config.Peer
	if err = json.Unmarshal(resp.Body, &peer); err != nil {
		t.Fatal(err)
	}

	fmt.Println(peer)

}

func TestGrpcClient_Register(t *testing.T) {
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051", Logger: log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "grpcclient"))})
	if err != nil {
		t.Fatal(err)
	}

	requset := &server.RegistryRequest{
		Hostname:            "test",
		Address:             "test",
		PersistentKeepalive: 25,
		PublicKey:           "test",
		PrivateKey:          "test",
		TieBreaker:          1,
		UpdatedAt:           time.Now(),
		CreatedAt:           time.Now(),
		Ufrag:               "test",
		Pwd:                 "test",
		Status:              1,
	}

	ctx := context.Background()
	body, err := json.Marshal(requset)

	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Registry(ctx, &pb.ManagementMessage{
		Body: body,
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get(ctx, &pb.ManagementMessage{Body: body})
	if err != nil {
		t.Fatal(err)
	}

	var peer config.Peer
	if err = json.Unmarshal(resp.Body, &peer); err != nil {
		t.Fatal(err)
	}

	fmt.Println(peer)
}
