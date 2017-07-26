.PHONY: deps build test

deps:
	glide i
	rm -rf ./vendor/github.com/docker/docker/vendor
	rm -rf ./vendor/github.com/docker/distribution/vendor

build: deps
	go build -ldflags "-s -w" -a -tags netgo -installsuffix netgo

test: deps
	go vet `go list ./... | grep -v '/vendor/'`
	go test -v `glide nv`
