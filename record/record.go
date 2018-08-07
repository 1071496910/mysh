package record

import (
	"encoding/json"
	"fmt"
	"github.com/1071496910/mysh/lru"
	"index/suffixarray"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

type Record interface {
	Find(s string) []string
	Add(s string) error
	List() []string
	//Dump() recordPersistentModel
}

func NewRcord(capacity int) Record {
	return &record{
		l: lru.NewLRU(capacity),
	}
}

type record struct {
	l lru.LRUCache
}

func (r *record) Find(s string) []string {
	kList, _ := r.l.Filter(func(key string, value interface{}) bool {
		return len(value.(*suffixarray.Index).Lookup([]byte(s), 1)) > 0
	})

	return kList
}

func (r *record) List() []string {
	kList, _ := r.l.Filter(func(key string, value interface{}) bool {
		return true
	})
	return kList
}

type recordPersistentModel struct {
	KList []string
	VList []string
}

//func (r *record) Dump() recordPersistentModel {
//
//	kList, vList := r.l.Filter(func(key string, value interface{}) bool {
//		return true
//	})
//
//	return recordPersistentModel{
//		KList: kList,
//		VList: vList,
//	}
//}

func (r *record) Add(s string) error {
	if s == "" {
		return fmt.Errorf("ERROR: record is empty")
	}

	if _, ok := r.l.Peek(s); ok {
		r.l.Contains(s) //增加权重
		return nil
	}

	index := suffixarray.New([]byte(s))
	r.l.Add(s, index)
	return nil

}

type PersistentRecord interface {
	Add(s string) error
	Find(s string) []string
	List() []string
	//Dump() recordPersistentModel
	Save() error
	Run()
}

func NewPersistentRecord(capacity int, f string) PersistentRecord {
	return &persistentRecord{
		inited: false,
		r:      NewRcord(capacity),
		f:      f,
	}
}

type persistentRecord struct {
	inited bool
	m      sync.Mutex
	r      Record
	f      string
}

func (p *persistentRecord) checkInited() {
	if !p.inited {
		log.Fatalf("record [%v] is not inited or load ", p.f)
	}

}

func (p *persistentRecord) Run() {

	p.initOrLoad()

	ticker := time.NewTicker(time.Second * 1)
	go func() {
		for _ = range ticker.C {
			p.sync()
		}
	}()
}

func (p *persistentRecord) sync() error {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	data, err := json.Marshal(p.r.List())
	if err != nil {
		log.Printf("parse [%v] records error...\n", p.f)
		return err
	}

	err = ioutil.WriteFile(p.f, data, 0644)
	if err != nil {
		log.Printf("sync [%v] records error...\n", p.f)
		return err
	}
	return nil
}

func (p *persistentRecord) Add(s string) error {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	if err := p.r.Add(s); err != nil {
		return err
	}

	return nil
}

func (p *persistentRecord) Find(s string) []string {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	return p.r.Find(s)
}

func (p *persistentRecord) List() []string {
	p.checkInited()
	p.m.Lock()
	defer p.m.Unlock()

	return p.r.List()
}

//func (p *persistentRecord) Dump() recordPersistentModel {
//	p.m.Lock()
//	defer p.m.Unlock()
//
//	return p.r.Dump()
//}

func (p *persistentRecord) Save() error {
	p.checkInited()
	return p.sync()
}

func (p *persistentRecord) initOrLoad() error {

	if _, err := os.Stat(p.f); os.IsNotExist(err) {

		f, err := os.Create(p.f)
		if err == nil {
			p.inited = true
			defer f.Close()
		}
		return err
	}

	data, err := ioutil.ReadFile(p.f)
	if err != nil {
		return err
	}

	recordObj := []string{}
	if len(data) == 0 {
		p.inited = true
		return nil
	}
	if err := json.Unmarshal(data, &recordObj); err != nil {
		return err
	}
	for i := len(recordObj) - 1; i >= 0; i-- {
		if err := p.r.Add(recordObj[i]); err != nil {
			return err
		}
	}

	p.inited = true
	return nil
}
