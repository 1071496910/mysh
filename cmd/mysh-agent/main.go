package main

import (
	"context"
	"fmt"
	"github.com/1071496910/mysh/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"log"
	"os"
	"path/filepath"
)

var recorder proto.SearchServiceClient

var logDir = "/var/log/mysh/"

var clientToken = ""

func init() {
	if err := os.MkdirAll(logDir, 0644); err != nil {
		panic(err)
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, "mysh-agent.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}

	newLogger := log.New(logFile, "[mysh-agent]", log.LstdFlags)

	grpclog.SetLogger(newLogger)

	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	recorder = proto.NewSearchServiceClient(conn)
}

func login() {
	resp, err := recorder.Login(context.Background(), &proto.LoginRequest{
		Uid:      "hpc",
		Password: "123456",
	})
	if err != nil {
		panic(err)
	}
	clientToken = resp.Token
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	login()

	command := os.Args[1]
	for i := 2; i < len(os.Args); i++ {
		command = command + " " + os.Args[i]
	}
	fmt.Println(recorder.Upload(context.Background(), &proto.UploadRequest{
		Token:  clientToken,
		Record: command,
		Uid:    "hpc",
	}))

	return

}
