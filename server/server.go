package server

import (
	"fmt"
	"github.com/1071496910/mysh/proto"
	"github.com/1071496910/mysh/recorder"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
)

type SearchServer struct {
	port int
}

func NewSearchServer(port int) *SearchServer {
	return &SearchServer{
		port: port,
	}

}

func (ss *SearchServer) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", ss.port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterSearchServiceServer(s, ss)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}

func (ss *SearchServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {

	return &proto.SearchResponse{
		Response: recorder.DefaultRecorderManager().Find(req.Uid, req.SearchString),
	}, nil
}

func (ss *SearchServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	if err := recorder.DefaultRecorderManager().Add(req.Uid, req.Record); err != nil {
		log.Printf("Add record uid[%v] command[%v] error: %v\n", req.Uid, req.Record, err)
		return &proto.UploadResponse{
			ErrorMsg:     fmt.Sprint(err),
			ResponseCode: 503,
		}, err

	}
	log.Printf("Add record uid[%v] command[%v] success!\n", req.Uid, req.Record)
	return &proto.UploadResponse{
		ResponseCode: 200,
	}, nil
}

func (ss *SearchServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	return nil, nil

}
