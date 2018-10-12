package main

//package etcd

import (
	"fmt"
	"time"
	"github.com/1071496910/mysh/lib/etcd"
)

func inc(iptr *int, threadId int) {
	for i := 0; i < 1000000; i++ {
		etcd.Lock("i")
		*iptr = *iptr + 1
		//time.Sleep(1 * time.Second)
		fmt.Printf("thread id: %v, i: %v\n", threadId, *iptr)
		etcd.UnLock("i")
	}
}

func main() {

	var i int

	go inc(&i, 1)
	go inc(&i, 2)
	fmt.Println("Finality: ", &i)

	time.Sleep(1000 * time.Second)
}
