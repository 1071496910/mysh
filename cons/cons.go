package cons

import "time"

var (
	Crt           = "/var/lib/mysh/cert/www.myshell.top.crt"
	Key           = "/var/lib/mysh/cert/www.myshell.top.key"
	Domain        = "www.myshell.top"
	Port          = 8080
	CertPort      = 8081
	MysqlStr      = "root:123456@tcp(localhost:3306)/mysh?tls=skip-verify&autocommit=true"
	MysqlTimeout  = 3 * time.Second
	UinfoTable    = "mysh_uinfo"
	UnixSocketDir = "/tmp"
	EnvMyshPidKey = "MYSH_PID"
)
