package main

import (
	"fmt"
	"index/suffixarray"
)

func main() {
	a := []byte("hello world")
	index := suffixarray.New(a)
	fmt.Println(index.Lookup([]byte(""), 1))

	fmt.Println("vim-go")
}
