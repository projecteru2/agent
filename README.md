Agent
======
[![CircleCI](https://circleci.com/gh/projecteru2/agent/tree/master.svg?style=shield)](https://circleci.com/gh/projecteru2/agent/tree/master)

Agent run on the node.

Features
========

* Forward log stream to remote
* Generate container metrics
* Bootstrap
* Auto monitor or remove containers

Build
========

##### build binary

```shell
make build
```

#### build rpm

```shell
./make-rpm
```

#### build docker

```shell
docker build -t agent .
```

Develop
========

```shell
go get github.com/projecteru2/agent.git
cd $GOPATH/src/get github.com/projecteru2/agent
make deps
```

Dockerized Agent
=================

```shell
docker run --rm --privileged -ti -e IN_DOCKER=1 \
  --name eru-agent --net host \
  -v /sys/fs/cgroup/:/sys/fs/cgroup/ \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc/:/hostProc/ \
  -v <HOST_CONFIG_PATH>:/etc/eru/agent.yaml \
  agent
```
