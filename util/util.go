package util

import (
	"github.com/1071496910/mysh/cons"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
