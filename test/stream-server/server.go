package main

import (
	"fmt"
	"github.com/1071496910/mysh/test/grpc-stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"log"
	"net"
	"time"
)

type streamServer struct {
	Port int
}

func (s *streamServer) Pause(pr *stream.PauseRequest, pcps stream.ProxyController_PauseServer) error {
	for {
		time.Sleep(time.Second)
		if err := pcps.Send(&stream.PauseResponse{ResponseCode: 200}); err != nil {
			log.Println(err)
			break
		}
	}
	return nil
}

func NewStreamServer(port int) *streamServer {
	return &streamServer{Port: port}
}

func RunSearchService(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Printf("init network error: %v", err)
		return err
	}

	s := grpc.NewServer()
	stream.RegisterProxyControllerServer(s, NewStreamServer(port))
	reflection.Register(s)

	if err := s.Serve(lis); err != nil {
		log.Println(err)
		return err
	}

	return nil

}

func main() {
	RunSearchService(8088)
}
