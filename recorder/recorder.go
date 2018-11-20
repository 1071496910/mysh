package recorder

import (
	"encoding/json"
	"fmt"
	"github.com/1071496910/mysh/lib/etcd"
	"github.com/1071496910/mysh/lru"
	"index/suffixarray"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	defaultFileStorageDir = "/var/lib/mysh/"
	//defaultFileRecorderSize = 10000
	defaultRecorderNum      = 100000
	defaultRecorderManager  RecorderManager
	defaultEtcdRecorderSize = 10000
	defaultEtcdPrefix       = "/mysh/data/"
)

func init() {
	defaultRecorderManager = &recorderManager{
		recorderSet: lru.NewLRU(defaultRecorderNum),
	}
}

type Recorder interface {
	Find(s string) []string
	Add(s string) error
	List() []string
	//Dump() recordPersistentModel
}

func NewRecorder(capacity int) Recorder {
	return &recorder{
		l: lru.NewLRU(capacity),
	}
}

type recorder struct {
	l lru.LRUCache
}

func (r *recorder) Find(s string) []string {
	kList, _ := r.l.Filter(func(key string, value interface{}) bool {
		return len(value.(*suffixarray.Index).Lookup([]byte(s), 1)) > 0
	})

	return kList
}

func (r *recorder) List() []string {
	kList, _ := r.l.Filter(func(key string, value interface{}) bool {
		return true
	})
	return kList
}

/*type recorderPersistentModel struct {
	KList []string
	VList []string
}*/

//func (r *recorder) Dump() recorderPersistentModel {
//
//	kList, vList := r.l.Filter(func(key string, value interface{}) bool {
//		return true
//	})
//
//	return recorderPersistentModel{
//		KList: kList,
//		VList: vList,
//	}
//}

func (r *recorder) Add(s string) error {
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

type PersistentRecorder interface {
	Add(s string) error
	Find(s string) []string
	List() []string
	//Dump() recordPersistentModel
	//Save() error
	Run()
	Stop()
}

//func NewFileRecorder(capacity int, f string) PersistentRecorder {
//	return &fileRecorder{
//		inited: false,
//		r:      NewRecorder(capacity),
//		f:      defaultFileStorageDir + f,
//	}
//}

func NewFileRecorder(capacity int, f string) PersistentRecorder {
	return &persistentRecorder{
		inited:      false,
		r:           NewRecorder(capacity),
		f:           defaultFileStorageDir + f,
		storageFunc: func(f string, data []byte) error { return ioutil.WriteFile(f, data, 0644) },
		tryInitFunc: func(f string) error {

			if _, err := os.Stat(f); os.IsNotExist(err) {

				f, err := os.Create(f)
				if err == nil {
					defer f.Close()
				}
				return err
			}
			return nil
		},
		loadFunc: func(f string) ([]byte, error) { return ioutil.ReadFile(f) },
	}
}

func NewEtcdRecorder(capacity int, f string) PersistentRecorder {
	return &persistentRecorder{
		inited:      false,
		r:           NewRecorder(capacity),
		f:           filepath.Join(defaultEtcdPrefix, f),
		stopCh:      make(chan interface{}, 1),
		storageFunc: func(f string, data []byte) error { return etcd.PutKV(f, string(data)) },
		tryInitFunc: func(f string) error { return nil },
		loadFunc:    func(f string) ([]byte, error) { return etcd.GetKV(f) },
	}

}

type persistentRecorder struct {
	inited      bool
	m           sync.Mutex
	r           Recorder
	f           string
	stopCh      chan interface{}
	storageFunc func(string, []byte) error
	tryInitFunc func(string) error
	loadFunc    func(string) ([]byte, error)
}

func (p *persistentRecorder) checkInited() {
	if !p.inited {
		log.Fatalf("record [%v] is not inited or load ", p.f)
	}
}

func (p *persistentRecorder) Run() {

	p.initOrLoad()

	ticker := time.NewTicker(time.Second * 1)
	go func() {
	Loop:
		for {
			select {
			case <-ticker.C:
				//log.Println("record run loop, before sync")
				p.sync()
				//log.Println("record run loop, after sync")
			case <-p.stopCh:
				log.Println("recorder stoping")
				break Loop
			}
		}
		log.Println("recorder", p.f, "finish")
	}()
}

func (p *persistentRecorder) Stop() {
	p.sync()
	log.Println("in persistent record stop(), after p.sync(), write to stop chan", p.stopCh)
	p.stopCh <- "Done"
	log.Println("in persistent record stop(), write stopch")
}

func (p *persistentRecorder) sync() error {
	p.checkInited()

	//log.Println("record.go in sync(), wait p.m.lock()")
	p.m.Lock()
	defer func() {
		//log.Println("record.go in sync(), release  p.m.lock()")
		p.m.Unlock()
	}()
	//log.Println("record.go in sync(), get p.m.lock()")

	data, err := json.Marshal(p.r.List())
	if err != nil {
		log.Printf("parse [%v] records error...\n", p.f)
		return err
	}

	err = p.storageFunc(p.f, data)
	//err = ioutil.WriteFile(p.f, data, 0644)
	if err != nil {
		log.Printf("sync [%v] records error...\n", p.f)
		return err
	}
	return nil
}

func (p *persistentRecorder) Add(s string) error {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	if err := p.r.Add(s); err != nil {
		return err
	}

	return nil
}

func (p *persistentRecorder) Find(s string) []string {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	return p.r.Find(s)
}

func (p *persistentRecorder) List() []string {
	p.checkInited()
	p.m.Lock()
	defer p.m.Unlock()

	return p.r.List()
}

//func (p *persistentRecorder) Dump() recordPersistentModel {
//	p.m.Lock()
//	defer p.m.Unlock()
//
//	return p.r.Dump()
//}

func (p *persistentRecorder) Save() error {
	p.checkInited()
	return p.sync()
}

func (p *persistentRecorder) initOrLoad() error {

	if err := p.tryInitFunc(p.f); err != nil {
		return err
	}
	p.inited = true

	data, err := p.loadFunc(p.f)
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

type RecorderManager interface {
	Add(id string, s string) error
	Find(id string, s string) []string
	Stop(id string) error
}

func DefaultRecorderManager() RecorderManager {
	return defaultRecorderManager
}

func NewRecorderManager() RecorderManager {
	return &recorderManager{
		recorderSet: lru.NewLRU(defaultRecorderNum),
	}
}

type recorderManager struct {
	mtx         sync.Mutex
	recorderSet lru.LRUCache
}

func (r *recorderManager) tryRun(id string) error {

	if _, ok := r.recorderSet.Peek(id); ok {
		return nil
	}

	recorder := NewEtcdRecorder(defaultEtcdRecorderSize, id)
	//recorder := NewFileRecorder(defaultFileRecorderSize, id)
	if recorder == nil {
		return fmt.Errorf("create file recorder error, id:[%v]", id)
	}

	recorder.Run()
	r.recorderSet.Add(id, recorder, func(i interface{}) {
		i.(PersistentRecorder).Stop()
	})

	return nil

}

func (r *recorderManager) Stop(id string) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	log.Println("record.go get recordermanager lock")
	if recorder, ok := r.recorderSet.Peek(id); ok {
		log.Println("DEBUG before  stop recorder")
		recorder.(PersistentRecorder).Stop()
		r.recorderSet.Remove(id)
		log.Println("DEBUG after stop recorder")
	}
	log.Println("record.go stop recorder fininsh")
	return nil
}

func (r *recorderManager) Add(id string, s string) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if err := r.tryRun(id); err != nil {
		return err
	}
	obj, _ := r.recorderSet.Get(id)
	return obj.(PersistentRecorder).Add(s)
}

func (r *recorderManager) Find(id string, s string) []string {
	//for connect check
	if id == "" && s == "" {
		return nil
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if err := r.tryRun(id); err != nil {
		return nil
	}

	obj, _ := r.recorderSet.Get(id)
	return obj.(PersistentRecorder).Find(s)
}

//old code

/*type fileRecorder struct {
	inited bool
	m      sync.Mutex
	r      Recorder
	f      string
	stopCh chan interface{}
}

func (p *fileRecorder) checkInited() {
	if !p.inited {
		log.Fatalf("record [%v] is not inited or load ", p.f)
	}

}

func (p *fileRecorder) Run() {

	p.initOrLoad()

	ticker := time.NewTicker(time.Second * 1)
	go func() {
		for _ = range ticker.C {
			select {
			case <-ticker.C:
				p.sync()
			case <-p.stopCh:
				p.Save()
				return
			}
		}
	}()
}

func (p *fileRecorder) Stop() {
	p.stopCh <- "Done"
}

func (p *fileRecorder) sync() error {
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

func (p *fileRecorder) Add(s string) error {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	if err := p.r.Add(s); err != nil {
		return err
	}

	return nil
}

func (p *fileRecorder) Find(s string) []string {
	p.checkInited()

	p.m.Lock()
	defer p.m.Unlock()

	return p.r.Find(s)
}

func (p *fileRecorder) List() []string {
	p.checkInited()
	p.m.Lock()
	defer p.m.Unlock()

	return p.r.List()
}

//func (p *fileRecorder) Dump() recordPersistentModel {
//	p.m.Lock()
//	defer p.m.Unlock()
//
//	return p.r.Dump()
//}

func (p *fileRecorder) Save() error {
	p.checkInited()
	return p.sync()
}

func (p *fileRecorder) initOrLoad() error {

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
}*/
