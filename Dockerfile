FROM golang:1.8-alpine AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

# copy code
COPY . /home/app
WORKDIR /home/app
# make binary
RUN apk add --no-cache git curl make \
    && curl https://glide.sh/get | sh \
    && make build \
    && ./agent --version

FROM alpine:3.6

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
COPY --from=BUILD /home/app/agent /usr/bin/eru-agent
COPY --from=BUILD /home/app/agent.yaml.sample /etc/eru/agent.yaml.sample
CMD eru-agent
