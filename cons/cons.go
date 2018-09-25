package cons

import (
	"log"
	"os/user"
	"path/filepath"
	"time"
)

func init() {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	UserCrt = filepath.Join(u.HomeDir, "mysh/cert/www.myshell.top.crt")
}

var (
	Crt          = "/var/lib/mysh/cert/www.myshell.top.crt"
	Key          = "/var/lib/mysh/cert/www.myshell.top.key"
	UserCrt      string
	Domain       = "www.myshell.top"
	Port         = 8080
	CertPort     = 8081
	DashPort     = 8082
	EndpointPort = 8083

	MysqlStr             = "root:123456@tcp(localhost:3306)/mysh?autocommit=true"
	LeaseTTL             = 10
	ProxyRegistryPrefix  = "/mysh/proxy/"
	ServerRegistryPrefix = "/mysh/server/"
	DashDataPrefix       = "/mysh/dash/"
	DashEpQueryFormat    = "/mysh/dash/uid/%v/endpoint"
	DashUidsQueryFormat  = "/mysh/dash/endpoint/%v/uids/%v"
	DashEpLock           = "/mysh/dash/eplock"
	WaitUidInterval      = time.Millisecond * 10
	//MysqlStr      = "root:123456@tcp(localhost:3306)/mysh?tls=skip-verify&autocommit=true"
	MysqlTimeout  = 3 * time.Second
	UinfoTable    = "mysh_uinfo"
	UnixSocketDir = "/tmp"
	EnvMyshPidKey = "MYSH_PID"
	EtcdEndpoints = "localhost:2379"
	EtcdTimeout   = 3 * time.Second
)
