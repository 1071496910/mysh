package main

import (
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
	"log"
)

func main() {
	log.Fatal(server.RunSearchService(cons.EndpointPort))
	//s := server.NewSearchServer(cons.EndpointPort)
	//log.Fatal(s.Run())
}
