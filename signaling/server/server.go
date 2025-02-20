package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"k8s.io/klog/v2"
	"linkany/management/grpc/client"
	"linkany/management/mapper"
	"linkany/pkg/drp"
	"linkany/pkg/linkerrors"
	"linkany/signaling/grpc/signaling"
	"net"
)

type Server struct {
	signaling.UnimplementedSignalingServiceServer
	listen      string
	userService mapper.UserInterface
	indexTable  *drp.IndexTable
	mgtClient   *client.Client

	forwardManager *ForwardManager
}

// Register will register a client to signaling server, will check token
func (s *Server) Register(ctx context.Context, message *signaling.EncryptMessage) (*signaling.EncryptMessage, error) {
	klog.Infof("register client: %v", message)
	var req signaling.EncryptMessageReqAndResp
	if err := proto.Unmarshal(message.Body, &req); err != nil {
		klog.Errorf("unmarshal failed: %v", err)
		return nil, err
	}

	_, err := s.mgtClient.VerifyToken(req.Token)
	if err != nil {
		klog.Errorf("verify token failed: %v", err)
		return nil, err
	}

	s.forwardManager.CreateChannel(message.PublicKey)
	klog.Infof("register '%v' client channel success", req.SrcPublicKey)

	var resp = &signaling.EncryptMessageReqAndResp{
		SrcPublicKey: req.SrcPublicKey,
		DstPublicKey: req.DstPublicKey,
	}

	body, err := proto.Marshal(resp)
	if err != nil {
		klog.Errorf("marshal failed: %v", err)
		return nil, err
	}

	return &signaling.EncryptMessage{
		Body: body,
	}, nil
}

func (s *Server) Forward(stream grpc.BidiStreamingServer[signaling.EncryptMessage, signaling.EncryptMessage]) error {

	done := make(chan interface{})
	defer func() {
		klog.Infof("close server signaling stream")
		close(done)
	}()

	req, err, body := s.recv(stream)
	if err != nil {
		return err
	}

	channel, b := s.forwardManager.GetChannel(req.SrcPublicKey)
	if !b {
		klog.Errorf("channel not exists: %v", req.SrcPublicKey)
		return errors.New("channel not exists")
	}

	go func() {
		for {
			select {
			case forwardMsg := <-channel:
				klog.Infof("forward message to client: %v", req.SrcPublicKey)
				if err := stream.Send(&signaling.EncryptMessage{Body: forwardMsg.Body}); err != nil {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.Canceled {
						klog.Infof("client canceled")
						return
					} else if err == io.EOF {
						klog.Infof("client closed")
						return
					}
					return
				}
			case <-done:
				s.forwardManager.DeleteChannel(req.SrcPublicKey) // because client closed
				klog.Infof("close forward signaling stream, delete channel: %v", req.SrcPublicKey)
				return
			}
		}
	}()

	klog.Infof("forward message: %v, body: %v", req.Type, body)
	s.forward(&req, body)

	for {
		req, err, body := s.recv(stream)
		if err != nil {
			return err
		}

		klog.Infof("forward message: %v, body: %v", req.Type, body)
		s.forward(&req, body)

		klog.Infof("forward message success")

	}
}

func (s *Server) forward(req *signaling.EncryptMessageReqAndResp, body []byte) {
	dstChannel, ok := s.forwardManager.GetChannel(req.DstPublicKey)
	if !ok {
		klog.Errorf("channel not exists: %v", req.DstPublicKey)
	}

	if dstChannel != nil {
		dstChannel <- &ForwardMessage{
			Body: body,
		}
	}

}

func (s *Server) recv(stream grpc.BidiStreamingServer[signaling.EncryptMessage, signaling.EncryptMessage]) (signaling.EncryptMessageReqAndResp, error, []byte) {
	msg, err := stream.Recv()
	if err != nil {
		s, ok := status.FromError(err)
		if ok && s.Code() == codes.Canceled {
			klog.Infof("client canceled")
			return signaling.EncryptMessageReqAndResp{}, linkerrors.ErrClientCanceled, nil
		} else if err == io.EOF {
			klog.Infof("client closed")
			return signaling.EncryptMessageReqAndResp{}, linkerrors.ErrClientClosed, nil
		}

		klog.Errorf("recv msg failed: %v", err)
		return signaling.EncryptMessageReqAndResp{}, err, nil
	}

	// forward message to client
	var req signaling.EncryptMessageReqAndResp
	if err := proto.Unmarshal(msg.Body, &req); err != nil {
		return signaling.EncryptMessageReqAndResp{}, err, nil
	}
	return req, nil, msg.Body
}

type ServerConfig struct {
	Port        int
	Listen      string
	UserService mapper.UserInterface
	Table       *drp.IndexTable
}

func NewServer(cfg *ServerConfig) (*Server, error) {

	mgtClient, err := client.NewClient(&client.GrpcConfig{
		Addr: "console.linkany.io:32051",
	})
	if err != nil {
		return nil, err
	}

	return &Server{
		mgtClient:      mgtClient,
		forwardManager: NewForwardManager(),
	}, nil
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", 32132))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	signaling.RegisterSignalingServiceServer(grpcServer, s)
	klog.Infof("Signaling grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}
