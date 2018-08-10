package auth

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
)

var defaultCertFile = "/data/.mysh/cert/mysh.cert"

func GetCert() (string, error) {
	data, err := ioutil.ReadFile(defaultCertFile)
	if err != nil {
		return nil, err
	}

	return string(data), nil

}

func genToken(uid string, extra ...string) string {

	data := uid
	for _, s := range extra {
		data = fmt.Sprintf("%v%v", data, s)
	}

	return string(md5.Sum([]byte(data)))

}
