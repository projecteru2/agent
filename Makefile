.PHONY: deps build test

deps:
	glide i
	rm -rf ./vendor/github.com/docker/docker/vendor
	rm -rf ./vendor/github.com/docker/distribution/vendor

build: deps
	go build -ldflags "-s -w" -a -tags netgo -installsuffix netgo

test: deps
	# fix mock docker client bug, see https://github.com/moby/moby/pull/34383 [docker 17.05.0-ce]
	sed -i.bak "143s/\*http.Transport/http.RoundTripper/" ./vendor/github.com/docker/docker/client/client.go
	go vet `go list ./... | grep -v '/vendor/'`
	go test -v `glide nv`
