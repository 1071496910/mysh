package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

func main() {
	cmd := exec.Command("/bin/bash") //启动命令为sh

	tty, err := os.Open("/dev/ptmx")
	if err != nil {
		log.Fatal(err)
	}

	//bashReadBuffer, err := cmd.StdinPipe()
	//if err != nil {
	//	log.Fatal(err)
	//}

	//bashWriteBuffer, err := cmd.StdoutPipe()
	//if err != nil {
	//	log.Fatal(err)
	//}

	//bashErrorBuffer, err := cmd.StderrPipe()
	//if err != nil {
	//	log.Fatal(err)
	//}

	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	//cmd.Stdin = bashReadBuffer
	//cmd.Stdout = bashWriteBuffer
	//cmd.Stderr = bashErrorBuffer
	//if err := cmd.Run(); err != nil {
	//	log.Fatal(err)
	//}
	//cmd.Stdin = os.Stdin
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr

	//if err := cmd.Start(); err != nil {
	//	println(err)
	//}

	go func() {
		for {
			io.Copy(tty, os.Stdin)
			//io.CopyN(bashReadBuffer, os.Stdin, 1)
			println("DEBUG: after in copy")
		}
	}()

	go func() {
		for {
			io.Copy(os.Stdout, tty)
			//io.CopyN(os.Stdout, bashWriteBuffer, 1)
			println("DEBUG: after out copy")
		}
	}()

	go func() {
		for {
			io.Copy(os.Stderr, tty)
			//io.CopyN(os.Stderr, bashErrorBuffer, 1)
			println("DEBUG: after error copy")
		}
	}()
	//if err := cmd.Wait(); err != nil {
	//	log.Fatal(err)
	//}
	cmd.Run()
	for {
		time.Sleep(1)
	}
}
