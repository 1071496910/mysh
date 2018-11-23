package util

import (
	"github.com/1071496910/mysh/cons"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func PullCert() error {
	resp, err := http.DefaultClient.Get("https://" + cons.Domain + ":443/get_cert")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	crtContent := []byte{}
	_, err = resp.Body.Read(crtContent)

	if err != nil {
		return err
	}
	baseDir := filepath.Dir(cons.UserCrt)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(cons.UserCrt)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	return err
}

func FileExist(f string) bool {
	if finfo, err := os.Stat(f); err == nil {
		return !finfo.IsDir()
	}

	return false
}

func AppendFile(fn string, data string) error {
	return writeFile(fn, data, func() (*os.File, error) {
		return os.OpenFile(fn, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	})
}

func OverWriteFile(fn string, data string) error {

	return writeFile(fn, data, func() (*os.File, error) {
		return os.Create(fn)
	})
}

func CheckTCP(endpint string) bool {
	if conn, err := net.Dial("tcp", endpint); err == nil {
		conn.Close()
		return true
	}
	return false
}

func writeFile(fn string, data string, openFunc func() (*os.File, error)) error {
	baseDir := filepath.Dir(fn)
	if err := os.MkdirAll(baseDir, 0644); err != nil {
		return err
	}

	f, err := openFunc()
	if err != nil {
		return err

	}
	_, err = f.WriteString(data)
	return err

}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return Retry(attempts, 2*sleep, f)
		}
		return err
	}

	return nil
}

type stop struct {
	error
}
