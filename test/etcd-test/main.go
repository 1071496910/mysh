package main

import (
	"github.com/1071496910/mysh/lib/etcd"
	"log"
	"strconv"
	"sync"
)

var (
	waitGroup sync.WaitGroup
	testKey   = "inc-test"
)

func doInc(wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 100; i++ {
		v, err := etcd.GetKV(testKey)
		if err != nil {
			log.Fatal(err)
		}

		i, err := strconv.Atoi(string(v))
		if err != nil {
			log.Fatal(err)
		}
		i++

		err = etcd.PutKV(testKey, strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}
	}

}

func main() {
	err := etcd.PutKV(testKey, "0")

	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go doInc(&waitGroup)
	}
	waitGroup.Wait()
	v, err := etcd.GetKV(testKey)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(v))
}
