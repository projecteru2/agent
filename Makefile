.PHONY: deps build test binary

REPO_PATH := github.com/projecteru2/agent
VERSION := $(shell git describe --tags $(shell git rev-list --tags --max-count=1))
GO_LDFLAGS ?= -s -w -X $(REPO_PATH)/common.EruAgentVersion=$(VERSION)

deps:
	env GO111MODULE=on go mod download
	env GO111MODULE=on go mod vendor

binary:
	go build -ldflags "$(GO_LDFLAGS)" -a -tags "netgo osusergo" -installsuffix netgo -o eru-agent

build: deps binary

test: deps
	# fix mock docker client bug, see https://github.com/moby/moby/pull/34383 [docker 17.05.0-ce]
	sed -i.bak "143s/\*http.Transport/http.RoundTripper/" ./vendor/github.com/docker/docker/client/client.go
	go vet `go list ./... | grep -v '/vendor/'`
	go test -v `go list ./... | grep -v '/vendor/'`
