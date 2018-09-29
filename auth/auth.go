package auth

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/db"
)

var (
	loginCache     = make(map[string]map[string]string)
	passwordGetter = db.MakePasswordGetter()
	loginCacheMtx  sync.Mutex
)

func CheckLoginState(uid string, token string, extra ...string) bool {

	loginInfo := ""
	for _, e := range extra {
		loginInfo = loginInfo + e
	}

	loginCacheMtx.Lock()
	defer loginCacheMtx.Unlock()

	if entry, ok := loginCache[uid]; ok {

		if t, eok := entry[loginInfo]; eok && t == token {
			return true
		}
	}
	log.Println("auth.go checklogin state invalid", uid, token, extra)
	return false
}

func RemoveTokenCache(uid string, extra ...string) error {
	loginInfo := ""
	for _, e := range extra {
		loginInfo = loginInfo + e
	}
	loginCacheMtx.Lock()
	defer loginCacheMtx.Unlock()
	if loginInfo == "" {
		delete(loginCache, uid)
	} else {
		if entry, ok := loginCache[uid]; ok {
			delete(entry, loginInfo)
		}
	}
	return nil

}

func UpdateTokenCache(uid string, token string, extra ...string) error {

	loginInfo := ""
	for _, e := range extra {
		loginInfo = loginInfo + e
	}

	loginCacheMtx.Lock()
	defer loginCacheMtx.Unlock()

	if loginInfo != "" {
		if _, ok := loginCache[uid]; !ok {
			loginCache[uid] = make(map[string]string)
		}
		loginCache[uid][loginInfo] = token
	}
	log.Println("auth.go update token cache:", loginCache)

	return nil

}

func GetCert() (string, error) {
	data, err := ioutil.ReadFile(cons.Crt)
	if err != nil {
		return "", err
	}

	return string(data), nil

}

func genToken(uid string, extra ...string) string {

	data := uid
	for _, s := range extra {
		data = fmt.Sprintf("%v%v", data, s)
	}
	tokenByte := md5.Sum([]byte(data))
	token := ""
	for _, b := range tokenByte {
		token = token + strconv.Itoa(int(b))
	}

	return token
}

func checkPassword(password string, saltPassword string) bool {
	return password == saltPassword
}

func Login(uid string, password string, extra ...string) (string, bool) {
	loginState := false
	token := ""
	if passwordGetter == nil {
		return "", false
	}
	p, err := passwordGetter(uid)
	if err != nil {
		log.Println(err)
		return "", false
	}
	if checkPassword(password, p) {
		loginState = true
	}

	if loginState {
		token = genToken(uid, extra...)
	}

	return token, loginState
}
