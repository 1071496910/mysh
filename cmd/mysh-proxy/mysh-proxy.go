package main

import (
	"log"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
)

func main() {
	s := server.NewProxyServer(cons.Port)
	log.Fatal(s.Run())
}
