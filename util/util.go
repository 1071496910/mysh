package util

import (
	"context"
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/proto"
	"google.golang.org/grpc"
	"os"
	"path/filepath"
	"strconv"
)

func PullCert() error {
	conn, err := grpc.Dial(cons.Domain+":"+strconv.Itoa(cons.CertPort), grpc.WithInsecure())
	if err != nil {
		return err
	}

	crtPuller := proto.NewCertServiceClient(conn)
	crt, err := crtPuller.Cert(context.Background(), &proto.CertRequest{})
	if err != nil {
		return err
	}
	return OverWriteFile(cons.Crt, crt.Content)
}

func FileExist(f string) bool {
	if finfo, err := os.Stat(f); err == nil {
		return !finfo.IsDir()
	}

	return false
}

func AppendFile(fn string, data string) error {
	return writeFile(fn, data, func() (*os.File, error) {
		return os.OpenFile(fn, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	})
}

func OverWriteFile(fn string, data string) error {

	return writeFile(fn, data, func() (*os.File, error) {
		return os.Create(fn)
	})
}

func writeFile(fn string, data string, openFunc func() (*os.File, error)) error {
	baseDir := filepath.Dir(fn)
	if err := os.MkdirAll(baseDir, 0644); err != nil {
		return err
	}

	f, err := openFunc()
	if err != nil {
		return err

	}
	_, err = f.WriteString(data)
	return err

}
