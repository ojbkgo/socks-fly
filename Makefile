
all: build-client-osx build-client-linux build-server-osx build-server-linux

build-client-osx:
	@echo "Building client for OSX"
	@GOOS=darwin GOARCH=arm64 go build -o bin/client-osx cmd/client/main.go
	@chmod +x bin/client-osx

build-client-linux:
	@echo "Building client for Linux"
	@GOOS=linux GOARCH=amd64 go build -o bin/client-linux cmd/client/main.go
	@chmod +x bin/client-linux

build-server-osx:
	@echo "Building server for OSX"
	@GOOS=darwin GOARCH=arm64 go build -o bin/server-osx cmd/server/main.go
	@chmod +x bin/server-osx

build-server-linux:
	@echo "Building server for Linux"
	@GOOS=linux GOARCH=amd64 go build -o bin/server-linux cmd/server/main.go
	@chmod +x bin/server-linux