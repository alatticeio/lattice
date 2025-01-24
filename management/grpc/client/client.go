package client

import (
	"context"
	"flag"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"linkany/management/grpc/mgt"
	"log"
	"time"
)

type GrpcConfig struct {
	Addr string
}

type GrpcClient struct {
	client mgt.ManagementServiceClient
}

//var (
//	addr = flag.String("addr", "localhost:50051", "the address to connect to")
//	name = flag.String("name", defaultName, "Name to greet")
//)

func NewGrpcClient(config *GrpcConfig) (*GrpcClient, error) {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.NewClient(config.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
		return nil, err
	}
	c := mgt.NewManagementServiceClient(conn)

	return &GrpcClient{client: c}, nil

	//// Contact the server and print out its response.
	//ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	//defer cancel()
	//stream, err := c.Watch(ctx, &pb.Request{Username: *name})
	//if err != nil {
	//	log.Fatalf("could not greet: %v", err)
	//}
	//
	//for {
	//	in, err := stream.Recv()
	//	if err == io.EOF {
	//		// read done.
	//		return
	//	}
	//	if err != nil {
	//		log.Fatalf("client watch failed: %v", err)
	//	}
	//	log.Printf("Got event: %v, peer: %v", in.Type, in.Peer)
	//}

}

func (c *GrpcClient) List(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	return c.client.List(ctx, in)
}

func (c *GrpcClient) Login(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	return c.client.Login(ctx, in)
}

func (c *GrpcClient) Watch(ctx context.Context, in *mgt.ManagementMessage, callback func(networkMap mgt.NetworkMap) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := c.client.Watch(ctx)
	if err != nil {
		log.Fatalf("client watch failed: %v", err)
	}

	if err = stream.Send(in); err != nil {
		log.Fatalf("client watch: stream.Send(%v) failed: %v", in, err)
	}

	ch := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				log.Fatalf("read done")
				close(ch)
				return
			}
			if err != nil {
				log.Fatalf("err: %v", err)
				continue
			}

			var networkMap mgt.NetworkMap
			if err := proto.Unmarshal(in.Body, &networkMap); err != nil {
				log.Fatalf("Failed to parse network map: %v", err)
				continue
			}

			callback(networkMap)
		}
	}()

	<-ch
	return nil
}
