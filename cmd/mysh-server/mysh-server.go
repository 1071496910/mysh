package main

import (
	"log"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
	"net/http"
)

func main() {
	s := server.NewSearchServer(cons.Port)
	cs := server.NewCertServer()
	http.Handle("/get_cert", cs)
	go http.ListenAndServeTLS(":443", cons.Crt, cons.Key, nil)
	log.Fatal(s.Run())
}
