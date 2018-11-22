package client

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/proto"
)

type Loginer interface {
	Login() (string, string)
}

type KVCache interface {
	Get(key string) string
	Set(key string, val string)
}

type mapCache struct {
	cache map[string]string
	rwmtx sync.RWMutex
}

func (mc mapCache) Get(key string) string {
	mc.rwmtx.RLock()
	defer mc.rwmtx.RUnlock()
	return mc.cache[key]
}

func (mc *mapCache) Set(key string, val string) {
	mc.rwmtx.Lock()
	defer mc.rwmtx.Unlock()
	mc.cache[key] = val
}

type envCache struct {
	rwmtx sync.RWMutex
}

func (ec envCache) Get(key string) string {
	ec.rwmtx.RLock()
	defer ec.rwmtx.RUnlock()
	return os.Getenv(key)
}

func (ec *envCache) Set(key string, val string) {
	ec.rwmtx.Lock()
	defer ec.rwmtx.Unlock()
	os.Setenv(key, val)
}

type loginer struct {
	kvCache KVCache
	client  proto.SearchServiceClient
	once    sync.Once
}

func (l *loginer) Login() (string, string) {
	l.once.Do(func() {
		l.kvCache.Set(cons.EnvKeyPs, "")
		l.kvCache.Set(cons.EnvKeyUid, "")
	})

	needLogin := false

	if l.kvCache.Get(cons.EnvKeyUid) == "" || l.kvCache.Get(cons.EnvKeyPs) == "" {
		needLogin = true
	} else {
		if resp, err := l.client.Login(context.Background(), &proto.LoginRequest{
			Uid:      l.kvCache.Get(cons.EnvKeyUid),
			Password: l.kvCache.Get(cons.EnvKeyPs),
		}); err == nil {
			return l.kvCache.Get(cons.EnvKeyUid), resp.Token
		}
		needLogin = true
	}

	for needLogin {

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter username: ")
		u, _, _ := reader.ReadLine()

		fmt.Print("Enter password: ")
		p, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		fmt.Println()
		l.kvCache.Set(cons.EnvKeyUid, string(u))
		l.kvCache.Set(cons.EnvKeyPs, string(p))

		resp, err := l.client.Login(context.Background(), &proto.LoginRequest{
			Uid:      string(u),
			Password: string(p),
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		return string(u), resp.Token
	}
	return "", ""

}

func NewLoginer(client proto.SearchServiceClient) Loginer {
	return &loginer{
		client: client,
		kvCache: &mapCache{
			cache: map[string]string{},
		},
	}
}
