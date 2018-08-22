package etcd

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"strings"
)

func PutKV(k, v string) error {

	endpoints := strings.Split(cons.EtcdEndpoints, ",")

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: cons.EtcdTimeout,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cons.EtcdTimeout)
	resp, err := cli.Put(ctx, k, v)
	cancel()
	if err != nil {
		return err
	}
	defer cli.Close()
}
