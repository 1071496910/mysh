package main

import (
	"github.com/1071496910/mysh/server"
	"log"
)

func main() {
	log.Fatal(server.RunSearchService(8084))
	//log.Fatal(server.RunSearchService(cons.EndpointPort))
	//s := server.NewSearchServer(cons.EndpointPort)
	//log.Fatal(s.Run())
}
