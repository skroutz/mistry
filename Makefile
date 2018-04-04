.PHONY: install build mistry mistry-cli test testall lint vet fmt clean

TESTCMD=go test -v -race

install: fmt vet test
	go install -v
	cd client && go install -v

build: mistry mistry-cli

mistry:
	go build -v -o mistry

mistry-cli:
	go build -v -o mistry-cli client/*.go

test: mistry-cli
	$(TESTCMD)

testall: test
	$(TESTCMD) --filesystem btrfs

lint:
	golint ./...

vet:
	go vet ./...

fmt:
	! gofmt -d -e -s *.go 2>&1 | tee /dev/tty | read

clean:
	go clean
