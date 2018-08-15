package main

import (
	"log"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
)

func main() {
	s := server.NewSearchServer(cons.Port)
	cs := server.NewCertServer(cons.CertPort)
	go cs.Run()
	log.Fatal(s.Run())
}
