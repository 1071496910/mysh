package main

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/1071496910/mysh/cons"
)

var (
	logDir = "/var/log/mysh/"
	l      *log.Logger
)

func init() {
	if err := os.MkdirAll(logDir, 0644); err != nil {
		panic(err)
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, "mysh-agent.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	l = log.New(logFile, "", log.LstdFlags)
}

func fakeDial(proto, addr string) (conn net.Conn, err error) {
	return net.Dial("unix", "/tmp/mysh."+os.Getenv(cons.EnvMyshPidKey)+".sock")
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	command := os.Args[1]
	for i := 2; i < len(os.Args); i++ {
		command = command + " " + os.Args[i]
	}

	tr := &http.Transport{
		Dial: fakeDial,
	}

	client := &http.Client{Transport: tr}
	if _, err := client.Post("http://xxx", "text/plain", bytes.NewBuffer([]byte(command))); err != nil {
		l.Println(err)
	}

	return

}
