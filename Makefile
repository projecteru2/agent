.PHONY: deps build test binary

REPO_PATH := github.com/projecteru2/agent
REVISION := $(shell git rev-parse HEAD || unknown)
BUILTAT := $(shell date +%Y-%m-%dT%H:%M:%S)
VERSION := $(shell git describe --tags $(shell git rev-list --tags --max-count=1))
GO_LDFLAGS ?= -X $(REPO_PATH)/version.REVISION=$(REVISION) \
			  -X $(REPO_PATH)/version.BUILTAT=$(BUILTAT) \
			  -X $(REPO_PATH)/version.VERSION=$(VERSION)
ifneq ($(KEEP_SYMBOL), 1)
	GO_LDFLAGS += -s
endif

all: build

deps:
	env GO111MODULE=on go mod download
	env GO111MODULE=on go mod vendor

binary:
	go build -ldflags "$(GO_LDFLAGS)" -o eru-agent

unit-test:
	go vet `go list ./... | grep -v '/vendor/'`
	go test `go list ./... | grep -v '/vendor/'`

build: deps binary

test: deps unit-test

lint:
	golangci-lint run

.PHONY: mock
mock:
	mockery --dir store --output store/mocks --name Store
