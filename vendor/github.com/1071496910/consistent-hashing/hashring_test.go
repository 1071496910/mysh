package hashring

import (
	"fmt"
	"testing"
)

func TestHashRing(t *testing.T) {
	hashRing, _ := NewHashRing(1024, 32)
	fmt.Println(hashRing.AddNode("127.0.0.1"))
	fmt.Println(hashRing.AddNode("127.0.0.1"))
	fmt.Println(hashRing.AddNode("127.0.0.2"))
	fmt.Println(hashRing.AddNode("127.0.0.3"))
	fmt.Println(hashRing.DelNode("127.0.0.1"))
	for i := 0; i < 100; i++ {
		node, err := hashRing.GetNode(fmt.Sprint(i))
		if err != nil {
			panic(err)
		}
		fmt.Printf("DEBUG: assign key %v to node %v\n", i, node)

	}
	fmt.Println(hashRing.AddNode("127.0.0.1"))
	for i := 0; i < 100; i++ {
		node, err := hashRing.GetNode(fmt.Sprint(i))
		if err != nil {
			panic(err)
		}
		fmt.Printf("DEBUG: assign key %v to node %v\n", i, node)

	}

}
