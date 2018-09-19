package main

import (
	"log"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
)

func main() {
	s := server.NewSearchServer(cons.EndpointPort)
	log.Fatal(s.Run())
}
