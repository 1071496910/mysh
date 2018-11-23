package main

import (
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
	"log"
)

func main() {

	s := server.NewDashServer(cons.DashAddr)
	//s := server.NewDashServer(":8085")

	go func() {
		dc, err := server.NewDashController()
		if err != nil {
			panic(err)
		}
		dc.Run()
	}()
	log.Fatal(s.Run())
}
