package hashring

import (
	"fmt"
	"hash/crc32"
	"sync"
)

type HashRing interface {
	AddNode(addr string) error
	DelNode(addr string) error
	GetNode(key string) (string,error)
}

type virtualNode struct {
	name       string
	symbolName string
	addr       string
}

type hashRing struct {
	nodeVNameMap map[string]uint32 //虚拟节点的name和slot的映射
	vnodeSlice   []*virtualNode    //slot大小的node的slice
	addedNode    map[string]byte   //记录已经存加入的node
	slotNum      int               //slot的个数
	vNodeNum     int
	rwMtx        sync.RWMutex
}

func NewHashRing(slotNum int, vNodeNum int) (*hashRing, error) {
	if slotNum <= 0 || vNodeNum <= 0 {
		return nil, fmt.Errorf("slotNum or vNodeNum can't less than 0.")
	}
	return &hashRing{
		nodeVNameMap: make(map[string]uint32),
		vnodeSlice:   make([]*virtualNode, slotNum),
		addedNode:    make(map[string]byte),
		slotNum:      slotNum,
		vNodeNum:     vNodeNum,
	}, nil

}

func (hr *hashRing) hashToSlot(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s)) % uint32(hr.slotNum)
}

func (hr *hashRing) AddNode(addr string) error {
	hr.rwMtx.Lock()
	defer hr.rwMtx.Unlock()
	if addr == "" {
		return fmt.Errorf("addr can't be empty string.")
	}

	if _, added := hr.addedNode[addr]; added {
		return fmt.Errorf("node already added.")
	}

	for i := 0; i < hr.vNodeNum; i++ {
		vname := fmt.Sprintf("%v--%v", addr, i)
		symbolName := ""
		var slotN uint32
		for j := 0; ; j++ {
			symbolName = fmt.Sprintf("%v-%v", vname, j)
			slotN = hr.hashToSlot(symbolName)
			if hr.vnodeSlice[slotN] == nil {
				hr.vnodeSlice[slotN] = &virtualNode{
					addr:       addr,
					name:       vname,
					symbolName: symbolName,
				}
				hr.nodeVNameMap[vname] = slotN
				break
			}

		}
	}
	hr.addedNode[addr] = 0
	return nil
}

func (hr *hashRing) DelNode(addr string) error {
	hr.rwMtx.Lock()
	defer hr.rwMtx.Unlock()
	if addr == "" {
		return fmt.Errorf("addr can't be empty string.")
	}
	if _, added := hr.addedNode[addr]; !added {
		return fmt.Errorf("node not exist when delete node.")
	}
	for i := 0; i < hr.vNodeNum; i++ {
		vname := fmt.Sprintf("%v--%v", addr, i)
		if slotN, ok := hr.nodeVNameMap[vname]; ok {
			hr.vnodeSlice[slotN] = nil
			delete(hr.nodeVNameMap, vname)
		}
	}
	delete(hr.addedNode, addr)

	return nil
}

func (hr *hashRing) GetNode(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key can't be empty string.")
	}
	keySlot := hr.hashToSlot(key)
	for i := keySlot; i < uint32(hr.slotNum); i = (i + 1) % uint32(hr.slotNum) {
		if vnode := hr.vnodeSlice[i]; vnode != nil {
			return vnode.addr, nil
		}
	}
	return "", fmt.Errorf("can't get node for key %v.", key)
}
