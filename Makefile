.PHONY: install build mistryd mistry test testall lint fmt clean

CLIENT=mistry
SERVER=mistryd
BUILDCMD=go build -v
TESTCMD=MISTRY_CLIENT_PATH="$(shell pwd)/$(CLIENT)" go test -v -race cmd/mistryd/*.go
TESTCLICMD=go test -v -race cmd/mistry/*.go

install: fmt test
	go install -v ./...

build: mistryd mistry

mistryd: generate
	$(BUILDCMD) -ldflags '-X main.VersionSuffix=$(shell git rev-parse HEAD)' -o $(SERVER) cmd/mistryd/*.go

mistry:
	$(BUILDCMD) -o $(CLIENT) cmd/mistry/*.go

test: generate mistry
	$(TESTCMD) --filesystem plain
	$(TESTCLICMD)

testall: test
	$(TESTCMD) --filesystem btrfs

deps:
	dep ensure -v

lint:
	golint `go list ./... | grep -v /vendor/`

fmt:
	! go fmt ./... 2>&1 | tee /dev/tty | read

clean:
	go clean ./...

generate:
	go generate ./...
