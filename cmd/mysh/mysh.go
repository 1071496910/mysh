package main
//test1
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc/credentials"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/proto"
	"github.com/1071496910/mysh/util"
	"github.com/1071496910/mysh/util/client"
)

func ctrl(b byte) byte {
	return b & 0x1f
}

func meta(b byte) byte {
	return b | 0x07f
}

var (
	ErrShortWrite = errors.New("short write")
	EOF           = errors.New("EOF")

	recorder proto.SearchServiceClient

	logDir string

	clientToken = ""
	loginer     func() string

	lastCommand = ""

	unixSock = ""
)

func init() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	logDir = usr.HomeDir + "/mysh/log"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(err)
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, "mysh.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}

	newLogger := log.New(logFile, "[mysh]", log.LstdFlags)

	grpclog.SetLogger(newLogger)

	if err := util.PullCert(); err != nil {
		panic(err)
	}
	// Create the client TLS credentials
	creds, err := credentials.NewClientTLSFromFile(cons.UserCrt, "")
	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(cons.Domain+":"+strconv.Itoa(cons.Port), grpc.WithTransportCredentials(creds))
	/*conn, err := grpc.Dial(cons.Domain+":"+strconv.Itoa(cons.Port), grpc.WithInsecure())*/
	if err != nil {
		panic(err)
	}
	recorder = proto.NewSearchServiceClient(conn)
	loginer = client.MakeVarLoginFunc(recorder)
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

func checkForeGround(pid int) bool {
	scriptContent := fmt.Sprintf("ps aux | awk '{if($2==%v)print}' | grep +", pid)
	cmd := exec.Command("/bin/bash", "-c", scriptContent)
	output, _ := cmd.CombinedOutput()
	if len(output) == 0 {
		return false
	}
	return true
}

func doSearch(stdinBuffer *bytes.Buffer, bashinBuffer *bytes.Buffer) {

	//check network
	resp, err := recorder.Search(context.Background(), &proto.SearchRequest{
		SearchString: "",
		Uid:          client.UidCache,
		Token:        clientToken,
	})

	//offline mode
	if err != nil {
		log.Println(err)
		bashinBuffer.WriteByte(ctrl('r'))

		for {
			time.Sleep(1 * time.Millisecond)
			b, err := stdinBuffer.ReadByte()
			if err == nil {
				bashinBuffer.WriteByte(b)

				if b == '\r' || b == '\n' {
					return
				}

			}
		}
	}

	//login again
	if resp.ResponseCode == 403 {
		login()
	}

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
					//candidateCommands = recorder.Find(string(searchBuffer))
					util.Retry(cons.ClientRetryTimes, cons.ClientRetryInterval, func() error {
						if resp, err := recorder.Search(context.Background(), &proto.SearchRequest{
							Uid:          client.UidCache,
							Token:        clientToken,
							SearchString: string(searchBuffer),
						}); err != nil {
							log.Println(err)
							return err
						} else if resp.ResponseCode != 200 {
							if resp.ResponseCode == 403 {
								login()
							}
							return errors.New("auth invalid")
						} else {
							candidateCommands = resp.Response
						}
						return nil
					})
				}
				displaySearchStatus()

			}
		}
	}
}

func Run() error {
	// Create arbitrary command.
	c := exec.Command("bash")
	c.Env = append(os.Environ(), `PROMPT_COMMAND=/usr/bin/mysh-agent $(history 1 | { read x cmd; echo "$cmd"; })`)

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	bashPid := c.Process.Pid

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
				if b == ctrl('r') && checkForeGround(bashPid) {
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

func login() {

	clientToken = loginer()
}

func uploadProxyHandler(resp http.ResponseWriter, req *http.Request) {
	body, _ := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if string(body) == lastCommand {
		return
	}

	rp, err := recorder.Search(context.Background(), &proto.SearchRequest{
		SearchString: "",
		Uid:          client.UidCache,
		Token:        clientToken,
	})

	//offline mode
	if err != nil {
		log.Println(err)
		return
	}

	//login again
	if rp.ResponseCode == 403 {
		login()
	}

	util.Retry(cons.ClientRetryTimes, cons.ClientRetryInterval, func() error {
		if resp, err := recorder.Upload(context.Background(), &proto.UploadRequest{
			Token:  clientToken,
			Record: string(body),
			Uid:    client.UidCache,
		}); err != nil {
			return err
		} else if resp.ResponseCode != 200 {
			if resp.ResponseCode == 403 {
				login()
			}
			return errors.New("auth invalid")
		}
		return nil
	})

	lastCommand = string(body)

}

func uploadProxyServer() {
	unixSock = filepath.Join(cons.UnixSocketDir, "mysh."+strconv.Itoa(os.Getpid())+".sock")

	// Start Server
	os.Remove(unixSock)
	unixListener, err := net.Listen("unix", unixSock)
	if err != nil {
		log.Fatal("Listen (UNIX socket): ", err)
	}
	defer unixListener.Close()
	log.Fatal(http.Serve(unixListener, http.HandlerFunc(uploadProxyHandler)))
}

func main() {
	//get token
	login()

	go uploadProxyServer()

	os.Setenv(cons.EnvMyshPidKey, strconv.Itoa(os.Getpid()))

	defer func() {
		logoutReq := &proto.LogoutRequest{
			Uid: client.UidCache,
		}

		if resp, err := recorder.Logout(context.Background(), logoutReq); err == nil && resp.ResponseCode == 200 {
			fmt.Println("Logout success!")
		}
	}()
	defer func() {
		if err := os.RemoveAll(unixSock); err != nil {
			fmt.Println(err)
		}
	}()

	if err := Run(); err != nil {
		log.Fatal(err)
	}

}
