//package main
package etcd

import (
	"context"
	"errors"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"google.golang.org/appengine/log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1071496910/mysh/cons"
)

var (
	cli  *clientv3.Client
	once sync.Once
)

var (
	ETCD_ERROR_EMPTY_VALUE = errors.New("Empty value")
)

func Init() {
	var err error
	endpoints := strings.Split(cons.EtcdEndpoints, ",")

	cli, err = clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: cons.EtcdTimeout,
	})
	if err != nil {
		panic(err)
	}
}

func PutKV(k, v string) error {
	once.Do(Init)

	ctx, cancel := context.WithTimeout(context.Background(), cons.EtcdTimeout)
	_, err := cli.Put(ctx, k, v)
	cancel()
	return err
}

func GetKV(k string) ([]byte, error) {
	once.Do(Init)

	ctx, cancel := context.WithTimeout(context.Background(), cons.EtcdTimeout)
	resp, err := cli.Get(ctx, k)
	cancel()
	if err != nil {
		return nil, err
	}
	for _, kv := range resp.Kvs {
		return kv.Value, nil
	}

	return nil, ETCD_ERROR_EMPTY_VALUE
}

var lockRecord map[string]clientv3.LeaseID = map[string]clientv3.LeaseID{}
var lockRecordMtx sync.Mutex

func tryLock(leaseId clientv3.LeaseID, k string) (bool, error) {
	t := cli.Txn(context.Background())
	//func(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)

	//txnResp, err := t.If(clientv3.Compare(clientv3.LeaseValue(k), "=", 0)).
	txnResp, err := t.If(clientv3.Compare(clientv3.CreateRevision(k), "=", 0)).
		Then(clientv3.OpPut(k, strconv.Itoa(int(leaseId)), clientv3.WithLease(leaseId))).Commit()
	if err != nil {
		return false, err
	}

	if !txnResp.Succeeded {
		//fmt.Println("try lock error")
		return false, err

	}
	fmt.Println("Get lease ok:", leaseId)

	return true, nil

}

func Lock(k string) error {
	once.Do(Init)
	leaseResp, err := cli.Grant(context.Background(), 10)
	if err != nil {
		return err
	}

	var lockSuccess bool = false

	for !lockSuccess {

		lockSuccess, err = tryLock(leaseResp.ID, k)
		if err != nil {
			return err
		}

		time.Sleep(time.Millisecond)
	}
	lockRecordMtx.Lock()
	lockRecord[k] = leaseResp.ID
	lockRecordMtx.Unlock()
	//clientv3.LeaseValue(k)
	return nil
}

func UnLock(k string) error {
	once.Do(Init)
	lockRecordMtx.Lock()
	defer lockRecordMtx.Unlock()
	if lockId, ok := lockRecord[k]; ok {
		_, err := cli.Revoke(context.Background(), lockId)
		if err != nil {
			return err
		}
		delete(lockRecord, k)
	}

	return fmt.Errorf("%v is already unlocked", k)
}

func Register(dir string, k string) error {
	once.Do(Init)
	leaseResp, err := cli.Grant(context.Background(), int64(cons.LeaseTTL))
	if err != nil {
		return err
	}
	cli.Put(context.Background(), filepath.Join(dir, k), fmt.Sprint(leaseResp.ID), clientv3.WithLease(leaseResp.ID))
	keepChan, err := cli.KeepAlive(context.Background(), leaseResp.ID)

	if err != nil {
		return err
	}

	go func() {
		//for v := range keepChan {
		for _ = range keepChan {
			//fmt.Println(v)
		}
	}()
	return nil
}

func ListKeyByPrefix(prefix string) ([]string, error) {
	once.Do(Init)

	resp, err := cli.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	ret := []string{}
	for _, kv := range resp.Kvs {
		ret = append(ret, string(kv.Key))
	}
	return ret, nil
}

func DElKeysByPrefix(prefix string)  error {
	once.Do(Init)

	_, err := cli.Delete(context.Background(), prefix, clientv3.WithPrefix())
	return err
}

const (
	OP_PUT = iota
	OP_DEL
)

type KV struct {
	Op    int
	Key   string
	Value string
}

func AtomicMultiKVOp(lockId string, kvs ...*KV) error {
	once.Do(Init)

	Lock(lockId)
	defer UnLock(lockId)

	ops := []clientv3.Op{}
	for _, kv := range kvs {
		switch kv.Op {
		case OP_PUT:
			ops = append(ops, clientv3.OpPut(kv.Key, kv.Value))
		case OP_DEL:
			ops = append(ops, clientv3.OpDelete(kv.Key))
		}
	}
	txn := cli.Txn(context.Background())
	_, err := txn.Then(ops...).Commit()
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

const (
	EVENT_ADD = iota
	EVENT_UPDATE
	EVENT_DEL
)

type event struct {
	key string
	val string
	et  int
}

type Controller struct {
	key        string //key for watch
	withPrefix bool
	eventCh    chan *event
	AddFunc    func(e *event) error
	UpdateFunc func(e *event) error
	DelFunc    func(e *event) error
}

func (c *Controller) Stop() {
	close(c.eventCh)
}

func (c *Controller) Run() {
	for e := range c.eventCh {
		switch e.et {
		case EVENT_ADD:
			if err := c.AddFunc(e); err != nil {
				log.Errorf(context.Background(), "deal create event error :%v \n", err)
			}
		case EVENT_UPDATE:
			if err := c.UpdateFunc(e); err != nil {
				log.Errorf(context.Background(), "deal update event error :%v \n", err)
			}
		case EVENT_DEL:
			if err := c.DelFunc(e); err != nil {
				log.Errorf(context.Background(), "deal delete event error :%v \n", err)
			}
		}
	}
}

func getInitReversion(key string) (int64, error) {
	resp, err := cli.Get(context.Background(), key, clientv3.WithPrefix())
	if err != nil {
		return -1, err
	}
	for _, kv := range resp.Kvs {
		fmt.Print("DEBUG: get kv ", kv.Key, kv.Value)
	}
	return resp.Header.Revision, nil
}

func WatchTest() {
	once.Do(Init)

	initReversion, err := getInitReversion("/")
	if err != nil {
		return
	}

	//cli.Do(context.Background(),clientv3.OpPut("1","2", clientv3.WithRev(1)))

	watchCh := cli.Watch(context.Background(), "/", clientv3.WithPrefix(), clientv3.WithRev(initReversion+1), clientv3.WithCreatedNotify())
	for es := range watchCh {
		for _, e := range es.Events {
			fmt.Println(e)
			fmt.Println(e.IsCreate())
		}

	}

}

/*func main() {
	WatchTest()
}*/
