package main

import (
	"context"
	"github.com/1071496910/mysh/test/grpc-stream"
	"google.golang.org/grpc"
	"log"
	"time"
)

func main() {
	conn, err := grpc.Dial("localhost:8088", grpc.WithInsecure())
	if err != nil {
		log.Println(err)
	}
	cli := stream.NewProxyControllerClient(conn)
	s, err := cli.Pause(context.Background(), &stream.PauseRequest{})
	if err != nil {
		log.Println(err)
	}
	for {
		time.Sleep(500 * time.Millisecond)
		i, err := s.Recv()
		if err != nil {
			log.Println(err)
			break
		}
		log.Println(i)
	}
}
