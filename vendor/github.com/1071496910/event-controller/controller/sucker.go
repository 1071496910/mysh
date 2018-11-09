package controller

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"strings"
	"sync"
	"time"
)

type EventSucker interface {
	GetSucker() (chan *Event, func(), error)
}

const (
	etcdSuckerConnTimeout = time.Second * 3
	etcdSuckerRecursive   = false
	etcdSuckerChanSize    = 1024
)

type EtcdSuckerEventObj struct {
	Key []byte
	Val []byte
}

type EtcdSucker struct {
	endpoints   string
	prefix      string
	recursive   bool
	connTimeout time.Duration
	chanSize    int

	once sync.Once
	cli  *clientv3.Client
}

type EtcdSuckerOption func(*EtcdSucker)

func EtcdSuckerWithRecursive() EtcdSuckerOption {
	return func(e *EtcdSucker) {
		e.recursive = true
	}
}

func EtcdSuckerConnTimeout(t time.Duration) EtcdSuckerOption {
	return func(e *EtcdSucker) {
		e.connTimeout = t
	}
}

func EtcdSuckerChanSize(len int) EtcdSuckerOption {
	return func(e *EtcdSucker) {
		e.chanSize = len
	}
}

func NewEtcdSucker(etcdEndpoints string, prefix string, opts ...EtcdSuckerOption) (*EtcdSucker, error) {
	etcdSucker := &EtcdSucker{
		endpoints:   etcdEndpoints,
		prefix:      prefix,
		chanSize:    etcdSuckerChanSize,
		connTimeout: etcdSuckerConnTimeout,
		recursive:   etcdSuckerRecursive,
	}

	for _, opt := range opts {
		opt(etcdSucker)
	}

	return etcdSucker, nil
}

func (es *EtcdSucker) GetSucker() (chan *Event, func(), error) {
	var initErr error
	var cli *clientv3.Client
	es.once.Do(func() {
		endpoints := strings.Split(es.endpoints, ",")

		cli, initErr = clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: es.connTimeout,
		})
		if initErr != nil {
			return
		}

	})

	if initErr != nil {
		return nil, nil, initErr
	}

	eventCh := make(chan *Event, es.chanSize)
	stopCh := make(chan int)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), es.connTimeout)
		clientV3Opts := []clientv3.OpOption{}
		if es.recursive {
			clientV3Opts = append(clientV3Opts, clientv3.WithPrefix())
		}

		resp, err := cli.Get(ctx, es.prefix, clientV3Opts...)
		cancel()
		if err != nil {
			return
		}

		for _, kv := range resp.Kvs {
			eventCh <- &Event{
				OPType: op_add,
				Obj: &EtcdSuckerEventObj{
					Key: kv.Key,
					Val: kv.Value,
				},
			}
		}

		watchOpts := []clientv3.OpOption{}
		watchOpts = append(watchOpts, clientv3.WithRev(resp.Header.Revision+1), clientv3.WithCreatedNotify())
		if es.recursive {
			watchOpts = append(watchOpts, clientv3.WithPrefix())
		}

		watchCh := cli.Watch(context.Background(), es.prefix, watchOpts...)
	watchLoop:
		for {
			select {
			case events := <-watchCh:
				for _, e := range events.Events {
					event := &Event{}
					if e.Type == 1 {
						event.OPType = op_del
					} else if e.IsCreate() {
						event.OPType = op_add
					} else {
						event.OPType = op_update
					}
					event.Obj = &EtcdSuckerEventObj{
						Key: e.Kv.Key,
						Val: e.Kv.Value,
					}
					eventCh <- event
				}

			case _ = <-stopCh:
				break watchLoop
			}
		}
	}()

	return eventCh, func() {
		close(stopCh)
	}, nil
}
