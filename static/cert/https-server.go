package main

import (
	"net/http"
)

func doRequest(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

func main() {
	http.ListenAndServeTLS(":443", "./full_chain.pem", "./private.key", http.HandlerFunc(doRequest))
}
