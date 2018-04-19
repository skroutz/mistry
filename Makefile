.PHONY: install build mistryd mistry test testall lint fmt clean

CLIENT=mistry
SERVER=mistryd
BUILDCMD=go build -v
TESTCMD=MISTRY_CLIENT_PATH="$(shell pwd)/$(CLIENT)" go test -v -race cmd/mistryd/*.go

install: fmt test
	go install -v ./...

build: mistryd mistry

mistryd: generate
	$(BUILDCMD) -o $(SERVER) cmd/mistryd/*.go

mistry:
	$(BUILDCMD) -o $(CLIENT) cmd/mistry/*.go

test: generate mistry
	$(TESTCMD) --filesystem plain

testall: test
	$(TESTCMD) --filesystem btrfs

deps:
	cd cmd/mistryd && dep ensure -v
	cd cmd/mistry && dep ensure -v

lint:
	golint ./...

fmt:
	! go fmt ./... 2>&1 | tee /dev/tty | read

clean:
	go clean ./...

generate:
	go generate ./...
