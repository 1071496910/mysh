package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh/terminal"
)

func test() error {
	// Create arbitrary command.
	c := exec.Command("bash")

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// Copy stdin to the pty and the pty to stdout.

	stdinBuffer := bytes.NewBuffer([]byte{})
	go func() {
		stdin := io.TeeReader(os.Stdin, stdinBuffer)
		_, _ = io.Copy(ptmx, stdin)
	}()
	go func() {
		line := []byte{}
		for {
			b, err := stdinBuffer.ReadByte()
			if err == nil {
				line = append(line, b)
				if strings.HasSuffix(string(line), "\r") {
					if len(string(line)) > 1 {
						record(string(line))
					}
					line = []byte{}
				}
			}
			time.Sleep(1 * time.Microsecond)

		}
	}()

	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}

var recordFile = "/tmp/bash-record"

func record(s string) error {

	f, err := os.OpenFile(recordFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	if _, err := f.WriteString(s); err != nil {
		log.Fatal(err)
	}

	return nil
}

func main() {
	if err := test(); err != nil {
		log.Fatal(err)
	}
}
