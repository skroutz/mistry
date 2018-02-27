.PHONY: install build test lint vet fmt clean

install: fmt vet test
	go install -v

test:
	go test -v -race ./...

lint:
	golint ./...

vet:
	go vet ./...

fmt:
	! gofmt -d -e -s *.go 2>&1 | tee /dev/tty | read

clean:
	go clean
