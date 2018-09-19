package main

//package etcd

import (
	"fmt"
	"time"
)

func inc(iptr *int, threadId int) {
	for i := 0; i < 1000000; i++ {
		Lock("i")
		*iptr = *iptr + 1
		//time.Sleep(1 * time.Second)
		fmt.Printf("thread id: %v, i: %v\n", threadId, *iptr)
		UnLock("i")
	}
}

func main() {

	var i int

	go inc(&i, 1)
	go inc(&i, 2)
	fmt.Println("Finality: ", &i)

	time.Sleep(1000 * time.Second)
}
