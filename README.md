## Local dev

```shell
go get gitlab.ricebook.net/platform/agent.git
mv $GOPATH/src/gitlab.ricebook.net/platform/agent.git $GOPATH/src/gitlab.ricebook.net/platform/agent
```

## Dockerized Agent

```sh
docker run --rm --privileged -ti -e IN_DOCKER=1 \
  --name eru-agent --net host \
  -v /sys/fs/cgroup/:/sys/fs/cgroup/ \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc/:/hostProc/ \
  -v <HOST_CONFIG_PATH>:/etc/eru/agent.yaml \
  hub.ricebook.net/platform/eruagent:<VERSION>
```
