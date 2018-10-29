package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	ch := make(chan int, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := range ch {
			time.Sleep(time.Second)
			fmt.Println("DEBUG:", i)
		}
	}()

	for i := 0; i < 10; i++ {
		ch <- i
	}
	close(ch)

	wg.Wait()
	fmt.Println("end")
}
