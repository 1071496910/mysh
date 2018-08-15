package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	//"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/proto"
	"github.com/1071496910/mysh/util/client"
)

var (
	recorder    proto.SearchServiceClient
	logDir      = "/var/log/mysh/"
	clientToken = ""
	loginer     func() string
)

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

	// Create the client TLS credentials
	creds, err := credentials.NewClientTLSFromFile(cons.Crt, "")
	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(cons.Domain+":8080", grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}
	recorder = proto.NewSearchServiceClient(conn)
	loginer = client.MakeEnvLoginFunc(recorder)
}

func login() {

	clientToken = loginer()
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
	recorder.Upload(context.Background(), &proto.UploadRequest{
		Token:  clientToken,
		Record: command,
		Uid:    os.Getenv(client.EnvKeyUid),
	})

	return

}
