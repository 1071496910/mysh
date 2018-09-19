package main

import (
	"github.com/1071496910/mysh/server"
	"log"
	"net/http"
)

func main() {

	cs := server.NewCertServer()
	http.Handle("/get_cert", cs)
	log.Fatal(http.ListenAndServe(":8081", nil))
}
