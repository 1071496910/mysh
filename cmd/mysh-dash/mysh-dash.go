package main

import (
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/server"
	"log"
	"time"
)

func main() {

	s := server.NewDashServer(cons.DashPort)

	go func() {
		time.Sleep(time.Second * 15)
		dc := server.NewDashController()
		for {
			time.Sleep(time.Second * 10)
			dc.Migrate("hpc", "[::]:8083", "[::]:8084")
			time.Sleep(time.Second * 10)
			dc.Migrate("hpc", "[::]:8084", "[::]:8083")
		}
	}()
	log.Fatal(s.Run())
}
