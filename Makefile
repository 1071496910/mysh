dev:
	cd cmd/mysh && go build && cd
	cd cmd/mysh-agent && go build && cp mysh-agent /usr/bin/ && cd
	cd cmd/mysh-server && go build && cd

