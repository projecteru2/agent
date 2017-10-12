Agent
======
[![CircleCI](https://circleci.com/gh/projecteru2/agent/tree/master.svg?style=shield)](https://circleci.com/gh/projecteru2/agent/tree/master)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/d13bd1a389244a77b0e11053025a963b)](https://www.codacy.com/app/CMGS/agent?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=projecteru2/agent&amp;utm_campaign=Badge_Grade)

Agent run on the node.

### Features

* Forward log stream to remote
* Generate container metrics
* Bootstrap
* Auto update containers' status and publish it by [core's](https://github.com/projecteru2/core) api.

### Build

#### build binary

`make build`

#### build rpm

`./make-rpm`

#### build image

`docker build -t agent .`

### Develop

```shell
go get github.com/projecteru2/agent.git
cd $GOPATH/src/get github.com/projecteru2/agent
make deps
```

### Dockerized Agent manually

```shell
docker run -d --privileged \
  --name eru_agent_$HOSTNAME \
  --net host \
  --restart always \
  -v /sys/fs/cgroup/:/sys/fs/cgroup/ \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc/:/hostProc/ \
  -v <HOST_CONFIG_DIR_PATH>:/etc/eru \
  projecteru2/agent \
  /usr/bin/eru-agent
```
