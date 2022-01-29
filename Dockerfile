FROM golang:alpine AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

# make binary
RUN apk add --no-cache git curl make gcc libc-dev
COPY . /go/src/github.com/projecteru2/agent
WORKDIR /go/src/github.com/projecteru2/agent
ARG KEEP_SYMBOL
RUN make build && ./eru-agent --version

FROM alpine:latest

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
LABEL ERU=1 agent=1
ENV AGENT_IN_DOCKER=1
COPY --from=BUILD /go/src/github.com/projecteru2/agent/eru-agent /usr/bin/eru-agent
COPY --from=BUILD /go/src/github.com/projecteru2/agent/agent.yaml.sample /etc/eru/agent.yaml.sample
