package auth

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"

	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/db"
)

var (
	loginCache     = make(map[string]string)
	passwordGetter = db.MakePasswordGetter()
)

func CheckLoginState(uid string, token string, extra ...string) bool {
	key := uid
	for _, e := range extra {
		key = key + e
	}

	if t, ok := loginCache[key]; ok && t == token {
		return true
	}
	return false
}

func RemoveTokenCache(uid string, extra ...string) error {
	key := uid
	for _, e := range extra {
		key = key + e
	}
	delete(loginCache, key)
	return nil

}

func UpdateTokenCache(uid string, token string, extra ...string) error {
	key := uid
	for _, e := range extra {
		key = key + e
	}

	loginCache[key] = token

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
