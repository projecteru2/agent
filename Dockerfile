FROM golang:1.9.0-alpine3.6 AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

# make binary
RUN apk add --no-cache git curl make \
    && curl https://glide.sh/get | sh \
    && go get -d github.com/projecteru2/agent
WORKDIR /go/src/github.com/projecteru2/agent
RUN make build && ./eru-agent --version

FROM alpine:3.6

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
LABEL ERU=1 version=latest agent=1
ENV AGENT_IN_DOCKER=1
COPY --from=BUILD /go/src/github.com/projecteru2/agent/eru-agent /usr/bin/eru-agent
COPY --from=BUILD /go/src/github.com/projecteru2/agent/agent.yaml.sample /etc/eru/agent.yaml.sample
