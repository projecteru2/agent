FROM golang:1.27-alpine AS build

RUN apk add --no-cache git make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG KEEP_SYMBOL
RUN make build && ./eru-agent --version

FROM alpine:3.22

LABEL ERU=1 agent=1
ENV AGENT_IN_DOCKER=1
RUN mkdir -p /etc/eru
COPY --from=build /src/eru-agent /usr/bin/eru-agent
COPY --from=build /src/agent.yaml.sample /etc/eru/agent.yaml.sample
