package main

import (
	"github.com/1071496910/mysh/server"
	"log"
)

func main() {
	s := server.NewSearchServer(8080)
	log.Fatal(s.Run())
}
