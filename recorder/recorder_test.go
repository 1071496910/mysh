package recorder

import (
	//"fmt"
	//"strconv"
	"testing"
	//"time"
)

func TestAdd(t *testing.T) {
	DefaultRecorderManager().Find("hello", "world")

}

//func TestAdd(t *testing.T) {
//
//	r := NewRcord(5)
//	r.Add("abcdefg")
//	r.Add("efgabcd")
//	r.Add("higklmn")
//	fmt.Println(r.Find("abc"))
//	fmt.Println(r.Find(""))
//
//}

//func TestPsAdd(t *testing.T) {
//	psr := NewPersistentRecorder(100001, "/tmp/tmp-record")
//	psr.Run()
//	psr.Add("higklmn")
//	for i := 0; i < 100000; i++ {
//
//		psr.Add("abcdefg" + strconv.Itoa(i))
//	}
//	start := time.Now()
//	psr.Find("abc")
//	elapsed := time.Since(start)
//	fmt.Println(elapsed)
//	fmt.Println(psr.Find("hig"))
//
//	for {
//	}
//
//}
