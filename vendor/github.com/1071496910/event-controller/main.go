package main

import (
	"fmt"

	"github.com/1071496910/event-controller/controller"
)

func main() {
	// func NewController(sucker controller.EventSucker, addFunc controller.EventDealFunc, updateFunc controller.EventDealFunc, delFunc controller.EventDealFunc, eventQueueLen
	// func NewEtcdSucker(etcdEndpoints string, prefix string, opts ...controller.EtcdSuckerOption) (*controller.EtcdSucker, error)
	sucker, err := controller.NewEtcdSucker("127.0.0.1:2379", "/", controller.EtcdSuckerWithRecursive())
	if err != nil {
		panic(err)
	}
	ctl := controller.NewController(
		sucker,
		func(i interface{}) {
			kv := i.(*controller.EtcdSuckerEventObj)
			fmt.Printf("In add func[key: %v, val: %v]\n", string(kv.Key), string(kv.Val))
		},
		func(i interface{}) {
			kv := i.(*controller.EtcdSuckerEventObj)
			fmt.Printf("In update func[key: %v, val: %v]\n", string(kv.Key), string(kv.Val))
		},
		func(i interface{}) {
			kv := i.(*controller.EtcdSuckerEventObj)
			fmt.Printf("In del func[key: %v, val: %v]\n", string(kv.Key), string(kv.Val))
		},
		1024)
	ctl.Run()

}
