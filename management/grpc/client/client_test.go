package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"linkany/internal"
	"linkany/management/entity"
	pb "linkany/management/grpc/mgt"
	"linkany/pkg/config"
	"sync"
	"testing"
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
}

func TestGrpcClient_List(t *testing.T) {
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051"})
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
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051"})
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
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051"})
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
	client, err := NewClient(&GrpcConfig{Addr: internal.ManagementDomain + ":32051"})
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
