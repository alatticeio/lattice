package client

import (
	"flag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "linkany/control/grpc/peer"
	"log"
)

type GrpcConfig struct {
	Addr string
}

//var (
//	addr = flag.String("addr", "localhost:50051", "the address to connect to")
//	name = flag.String("name", defaultName, "Name to greet")
//)

func NewGrpcClient(config *GrpcConfig) (pb.ListWatcherClient, error) {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.NewClient(config.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
		return nil, err
	}
	c := pb.NewListWatcherClient(conn)

	return c, nil

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
