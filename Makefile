.PHONY: deps build test

deps:
	go get -d -v github.com/Sirupsen/logrus
	go get -d -v github.com/bmizerany/pat
	go get -d -v github.com/CMGS/statsd
	go get -d -v gopkg.in/yaml.v2
	go get -d -v github.com/coreos/etcd
	go get -d -v github.com/docker/docker/pkg/stdcopy
	go get -d -v gopkg.in/urfave/cli.v1
	go get -d -v github.com/docker/docker/client
	go get -d -v github.com/docker/go-unit
	go get -d -v github.com/docker/go-connections
	go get -d -v github.com/docker/distribution/reference
	go get -d -v github.com/opencontainers/runc/libcontainer/user
	rm -rf $GOPATH/src/github.com/docker/docker/vendor

build:
	go build -ldflags "-s -w" -a -tags netgo -installsuffix netgo

test:
	go tool vet .
	go test -v ./...
	golint ./...
