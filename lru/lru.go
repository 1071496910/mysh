package lru

import (
	"container/list"
	"fmt"
	"sync"
)

// LRUCache is the interface for simple LRU cache.
type LRUCache interface {
	// Adds a value to the cache, returns true if an eviction occurred and
	// updates the "recently used"-ness of the key.
	Add(key, value interface{}) bool

	// Returns key's value from the cache and
	// updates the "recently used"-ness of the key. #value, isFound
	Get(key interface{}) (value interface{}, ok bool)

	// Check if a key exsists in cache without updating the recent-ness.
	Contains(key interface{}) (ok bool)

	// Returns key's value without updating the "recently used"-ness of the key.
	Peek(key interface{}) (value interface{}, ok bool)

	// Removes a key from the cache.
	Remove(key interface{}) bool

	// Removes the oldest entry from cache.
	RemoveOldest() (interface{}, interface{}, bool)

	// Returns the oldest entry from the cache. #key, value, isFound
	GetOldest() (interface{}, interface{}, bool)

	// Returns a slice of the keys in the cache, from oldest to newest.
	Keys() []interface{}

	// Returns the number of items in the cache.
	Len() int

	// Clear all cache entries
	Purge()

	// Filter
	Filter(filterFunc func(key string, value interface{}) bool) ([]string, []interface{})
}

func NewLRU(capacity int) LRUCache {
	if capacity < 1 {
		panic("capacity num is invalid")
	}

	return &lru{
		capacity: capacity,
		l:        list.New(),
		m:        make(map[string]*list.Element),
	}
}

type lru struct {
	m        map[string]*list.Element
	l        *list.List
	mtx      sync.Mutex
	capacity int
}

type node struct {
	key   interface{}
	value interface{}
}

func (l *lru) Filter(filterFunc func(key string, value interface{}) bool) ([]string, []interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	kList := make([]string, 0)
	vList := make([]interface{}, 0)

	for tmp := l.l.Front(); tmp != nil; tmp = tmp.Next() {
		if filterFunc(tmp.Value.(*node).key.(string), tmp.Value.(*node).value) {
			kList = append(kList, tmp.Value.(*node).key.(string))
			vList = append(vList, tmp.Value.(*node).value)
		}
	}

	return kList, vList

}

func (l *lru) Add(key, value interface{}) bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	evicted := false
	keyString := fmt.Sprint(key)

	//1. 已经存在更新value
	//2. 不存在，判断list是否到达capacity
	//2.1 到达capacity，删除最旧的cache，插入新的cache
	//2.2 没到达capacity，直接插入
	if item, ok := l.m[keyString]; ok {
		item.Value = &node{
			key:   keyString,
			value: value,
		}
		l.l.MoveToFront(item)
	} else {
		if l.l.Len() == l.capacity {
			lastNode := l.l.Back()
			l.l.Remove(lastNode)
			l.l.Back()
			v, ok := lastNode.Value.(*node)
			if !ok {
				panic("type error")

			}
			delete(l.m, fmt.Sprint(v.key))
			evicted = true

		}
		e := l.l.PushFront(&node{
			key:   key,
			value: value,
		})

		l.m[keyString] = e
	}
	return evicted

}

func (l *lru) commonGet(key interface{}, peek bool) (value interface{}, ok bool) {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	if l.l.Len() == 0 {
		return nil, false
	}

	keyString := fmt.Sprint(key)
	ok = true
	value = nil
	v, exists := l.m[keyString]
	if exists {
		vn, right := v.Value.(*node)
		if !right {
			panic("type error")
		}
		if vn != nil {
			value = vn.value
		}

		if !peek && l.l.Len() > 1 {
			l.l.MoveBefore(v, l.l.Front())
		}

	} else {
		ok = false
	}

	return

}

func (l *lru) Get(key interface{}) (value interface{}, ok bool) {
	value, ok = l.commonGet(key, false)
	return

}

// Check if a key exsists in cache without updating the recent-ness.
func (l *lru) Contains(key interface{}) (ok bool) {
	_, ok = l.Get(key)
	return

}

// Returns key's value without updating the "recently used"-ness of the key.
func (l *lru) Peek(key interface{}) (value interface{}, ok bool) {
	value, ok = l.commonGet(key, true)
	return
}

// Removes a key from the cache.
func (l *lru) Remove(key interface{}) bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if l.l.Len() == 0 {
		return false
	}

	keyString := fmt.Sprint(key)
	if v, ok := l.m[keyString]; ok {

		l.l.Remove(v)
		delete(l.m, keyString)
		return true

	}

	return false
}

// Removes the oldest entry from cache.
func (l *lru) RemoveOldest() (interface{}, interface{}, bool) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if l.l.Len() == 0 {
		return nil, nil, false
	}
	tail := l.l.Back()
	if tailNode, ok := tail.Value.(*node); ok {
		keyString := fmt.Sprint(tailNode.key)
		delete(l.m, keyString)
		l.l.Remove(tail)
		return tailNode.key, tailNode.value, true

	} else {
		panic("type error")
	}

}

// Returns the oldest entry from the cache. #key, value, isFound
func (l *lru) GetOldest() (interface{}, interface{}, bool) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if l.l.Len() == 0 {
		return nil, nil, false
	}
	tail := l.l.Back()
	if tailNode, ok := tail.Value.(*node); ok {
		return tailNode.key, tailNode.value, true
	} else {
		panic("type error")
	}
}

// Returns a slice of the keys in the cache, from oldest to newest.
func (l *lru) Keys() []interface{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	res := make([]interface{}, l.l.Len())
	head := l.l.Front()
	for i := 0; i < l.l.Len(); i++ {
		if node, ok := head.Value.(*node); ok {
			res = append(res, node.key)
		} else {
			panic("pyte error")
		}
	}
	return res

}

// Returns the number of items in the cache.
func (l *lru) Len() int {
	return l.l.Len()
}

// Clear all cache entries
func (l *lru) Purge() {
	l.l = list.New()
	l.m = make(map[string]*list.Element)

}
