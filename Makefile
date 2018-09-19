binary:
	cd cmd/mysh && go build -i && cd -
	cd cmd/mysh-agent && go build -i && cd -
	cd cmd/mysh-server && go build -i && cd -
	cd cmd/mysh-cert-server && go build -i && cd -
	cd cmd/mysh-dash && go build -i && cd -
	cd cmd/mysh-proxy && go build -i && cd -

local-install:
	cd cmd/mysh-agent && sudo cp mysh-agent /usr/bin/ && cd -

dev-deploy:
	cd cmd/mysh-agent && cp mysh-agent /usr/bin/ && cd -
	cd cmd/mysh-server &&  scp mysh-server www.myshell.top:~/  && cd -

