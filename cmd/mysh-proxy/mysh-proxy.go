package main

import (
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
	"log"
)

func main() {
	//s := server.NewProxyServer(cons.Port)
	log.Fatal(server.RunProxyService(cons.Port))
	//log.Fatal(s.Run())
}
