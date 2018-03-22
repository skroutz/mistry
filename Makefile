.PHONY: install build test lint vet fmt clean

install: fmt vet test
	go install -v

test:
	# TODO: enable -race and -v
	go test --config config.test.json --filesystem plain

testall: test
	go test --config config.test.json --filesystem btrfs

lint:
	golint ./...

vet:
	go vet ./...

fmt:
	! gofmt -d -e -s *.go 2>&1 | tee /dev/tty | read

clean:
	go clean
