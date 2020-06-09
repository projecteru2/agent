Agent
======
![](https://github.com/projecteru2/cli/workflows/goreleaser/badge.svg)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/0ae492f3a2c04f7c9b743998f4184943)](https://www.codacy.com/gh/projecteru2/agent?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=projecteru2/agent&amp;utm_campaign=Badge_Grade)

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
go get github.com/projecteru2/agent
cd $GOPATH/src/get github.com/projecteru2/agent
make deps
```

### Dockerized Agent manually

```shell
docker run -d --privileged \
  --name eru_agent_$HOSTNAME \
  --net host \
  --restart always \
  -v /sys:/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc/:/hostProc/ \
  -v <HOST_CONFIG_DIR_PATH>:/etc/eru \
  projecteru2/agent \
  /usr/bin/eru-agent
```

### Build and Deploy by Eru itself

After we implemented bootstrap in eru2, now you can build and deploy agent with [cli](https://github.com/projecteru2/cli) tool.

1. Test source code and build image

```shell
<cli_execute_path> --name <image_name> http://bit.ly/EruAgent
```

Make sure you can clone code by ssh protocol because libgit2 ask for it. So you need configure core with github certs. After the fresh image was named and tagged, it will be auto pushed to the remote registry which was defined in core.

2. Deploy agent by eru with specific resource.

```shell
<cli_execute_path> container deploy -pod <pod_name> --entry agent --network <network_name> --deploy-method fill --image <projecteru2/agent>|<your_own_image> --count 1 --file <agent_config_yaml>:/etc/eru/agent.yaml [--cpu 0.3 | --mem 1024000000] http://bit.ly/EruAgent
```

Now you will find agent was started in each node, and monitor containers status include itself.
