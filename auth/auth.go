package auth

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"strconv"
)

var defaultCertFile = "/var/lib/mysh/cert/www.mysh.cn.crt"

var passwordCache = make(map[string]string)

var loginCache = make(map[string]string)

func init() {
	passwordCache["hpc"] = "123456"
	passwordCache["hpc2"] = "2123456"
	passwordCache["hpc3"] = "3123456"
	passwordCache["hpc4"] = "4123456"
}

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
	fmt.Println(loginCache)

	return nil

}

func GetCert() (string, error) {
	data, err := ioutil.ReadFile(defaultCertFile)
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
	loginSuccess := false
	token := ""
	if p, ok := passwordCache[uid]; ok {
		if checkPassword(password, p) {
			loginSuccess = true
		}
	} else {
		//TODO: read password and compare
	}

	if loginSuccess {
		token = genToken(uid, extra...)
	}

	return token, loginSuccess
}
