package common

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	//"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	//"strconv"
	//"strings"
	"syscall"
	"time"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/1071496910/mysh/recorder"
)

func ctrl(b byte) byte {
	return b & 0x1f
}

func meta(b byte) byte {
	return b | 0x07f
}

var ErrShortWrite = errors.New("short write")
var EOF = errors.New("EOF")
var Recorder = recorder.NewFileRecorder(100000, "/tmp/hpc/bash-record")

func init() {
	Recorder.Run()
}

func streamCopy(dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	var err error

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != EOF {
				err = er
			}
			break
		}
	}
	return written, err

}

func printCSI(c []byte) {
	seq := append([]byte{0x1b, '['}, c...)
	os.Stdout.Write(seq)
}

func saveCursor() {
	printCSI([]byte{'s'})
}

func restoreCursor() {
	printCSI([]byte{'u'})
}

func cleanLineTail() {
	printCSI([]byte{'K'})
}

func doSearch(stdinBuffer *bytes.Buffer, bashinBuffer *bytes.Buffer) {
	saveCursor()

	lastView := ""
	cleanView := func() {

		restoreCursor()
		cleanLineTail()
	}

	candidateCommands := make([]string, 0)
	candidateCommandsIndex := 0

	searchBuffer := make([]byte, 0, 1024)
	searchIndex := 0

	displaySearchStatus := func() {
		cleanView()
		if candidateCommandsIndex >= 0 && candidateCommandsIndex < len(candidateCommands) {
			lastView = candidateCommands[candidateCommandsIndex]
		}

		fmt.Printf("C-R mode: ( %v ) %v", string(searchBuffer), lastView)

		//println("DEBUG: ", lastView)
		//bashinBuffer.Write([]byte(lastView))
	}

	displayCommand := func() {
		cleanView()
		if candidateCommandsIndex >= 0 && candidateCommandsIndex < len(candidateCommands) {
			lastView = candidateCommands[candidateCommandsIndex]
		}
		bashinBuffer.Write([]byte(lastView))
	}

	displaySearchStatus()

	for {
		time.Sleep(1 * time.Millisecond)
		b, err := stdinBuffer.ReadByte()
		//if err != nil && err != EOF {
		//	cleanView()
		//	panic(err)
		//}

		if err == nil {
			switch b {
			case ctrl('c'):
				cleanView()
				return
			case '\r':
				displayCommand()
				return
			case ctrl('r'):
				candidateCommandsIndex = candidateCommandsIndex - 1
				if candidateCommandsIndex < 0 {
					candidateCommandsIndex = len(candidateCommands) - 1
				}
				displaySearchStatus()

			case 0x1b:
				b, err := stdinBuffer.ReadByte()
				if err != nil {
					panic(err)
				}

				if b != '[' {
					panic("unknow CSI sequence")
				}

				//fileLog("DEBUG: in CSI seq")
				c, err := stdinBuffer.ReadByte()
				if err != nil {
					panic(err)
				}
				if c == 'C' || c == 'D' {
					displayCommand()
					return
				}

				if c == 'A' {
					candidateCommandsIndex = candidateCommandsIndex - 1
					if candidateCommandsIndex < 0 {
						candidateCommandsIndex = len(candidateCommands) - 1
					}
					displaySearchStatus()

				}

				if c == 'B' {
					candidateCommandsIndex = candidateCommandsIndex + 1
					if candidateCommandsIndex >= len(candidateCommands) {
						candidateCommandsIndex = 0
					}
					displaySearchStatus()
				}

			default:
				candidateCommandsIndex = 0
				if b == 127 { //backspace

					searchIndex = searchIndex - 1

					if searchIndex < 1 {
						candidateCommands = []string{
							"",
						}
						searchBuffer = []byte{}
						searchIndex = 0

					} else {
						searchBuffer = searchBuffer[0:searchIndex]
					}

				} else {
					searchIndex = searchIndex + 1
					searchBuffer = append(searchBuffer, b)

				}
				//println("DEBUG: search buffer is ", string(searchBuffer))
				if searchIndex > 0 {
					candidateCommands = Recorder.Find(string(searchBuffer))
				}
				displaySearchStatus()

			}
		}
	}
}

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
	bashinBuffer := bytes.NewBuffer([]byte{})
	go func() {
		for {
			time.Sleep(1 * time.Millisecond)
			_, _ = streamCopy(stdinBuffer, os.Stdin)
		}
	}()

	go func() {
		for {
			time.Sleep(1 * time.Millisecond)
			b, err := stdinBuffer.ReadByte()
			if err == nil {
				if b == ctrl('r') {

					doSearch(stdinBuffer, bashinBuffer)
					continue
				}
				bashinBuffer.WriteByte(b)
				continue
			}
		}
	}()

	go func() {
		for {
			time.Sleep(1 * time.Millisecond)
			_, _ = streamCopy(ptmx, bashinBuffer)
		}
	}()

	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}

func main() {
	if err := test(); err != nil {
		log.Fatal(err)
	}
}
