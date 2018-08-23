package etcd

import (
	"context"
	"errors"
	"github.com/coreos/etcd/clientv3"
	"strings"
	"sync"

	"github.com/1071496910/mysh/cons"
)

var (
	cli  *clientv3.Client
	once sync.Once
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

	return nil, errors.New("Empty value")
}
