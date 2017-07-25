.PHONY: deps build test

deps:
	glide i

build:
	go build -ldflags "-s -w" -a -tags netgo -installsuffix netgo

test:
	go tool vet .
	go test -v ./...
	golint ./...
