package main

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

func main() {
	fmt.Println(terminal.IsTerminal(int(os.Stdin.Fd())))
	fmt.Println(terminal.IsTerminal(int(os.Stdout.Fd())))
	fmt.Println(terminal.IsTerminal(int(os.Stderr.Fd())))
}
