package main

import (
	"github.com/1071496910/mysh/recorder"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		panic("not enough args")
	}

	recorder := recorder.NewFileRecorder(100000, "/tmp/hpc/bash-record")
	recorder.Run()
	command := os.Args[1]
	for i := 2; i < len(os.Args); i++ {
		command = command + " " + os.Args[i]
	}
	recorder.Add(command)
	recorder.Save()

	return

}
