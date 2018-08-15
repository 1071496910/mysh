package util

import (
	"testing"
)

//func TestFileCheck(t *testing.T) {
//	t.Log(FileExist("/tmp/tmp"))
//
//}

func TestFileWrite(t *testing.T) {
	t.Log(OverWriteFile("/tmp/tmp", "test"))
	t.Log(AppendFile("/tmp/tmp", "test"))
	t.Log(AppendFile("/tmp/tmp", "test"))
}
