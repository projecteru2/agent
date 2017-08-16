FROM golang:1.9.0-alpine3.6 AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

# make binary
RUN apk add --no-cache git curl make \
    && curl https://glide.sh/get | sh \
    && go get -d github.com/projecteru2/agent \
    && cd src/github.com/projecteru2/agent \
    && make build \
    && ./eru-agent --version

FROM alpine:3.6

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
COPY --from=BUILD /go/src/github.com/projecteru2/agent/eru-agent /usr/bin/eru-agent
COPY --from=BUILD /go/src/github.com/projecteru2/agent/agent.yaml.sample /etc/eru/agent.yaml.sample
