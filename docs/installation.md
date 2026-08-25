# Installation

`eru-agent` is a single static binary. It needs a reachable `eru-core` and, for each runtime it is configured with, that runtime on the same host: a Docker socket, a containerd socket, or systemd with cgroup v2 plus `journalctl`.

The binary also carries the helper modes containerd invokes on the node, neither of them started by hand. `eru-agent log-shim` is the binary logger core points a task's `cio.LogURI` at. `eru-agent oci-hook --network <cni network> [--conf-dir /etc/cni/net.d] [--bin-dir /opt/cni/bin]` is what core writes into a container's OCI spec, as a `createRuntime` hook and again as a `poststop` hook; it needs the CNI plugin binaries and the containerd socket on the node.

## From source

```bash
git clone https://github.com/projecteru2/agent.git
cd agent
make build
./eru-agent --version
```

`make build` produces `eru-agent` with `CGO_ENABLED=0`, stamping version, revision and build time into `version/`. Set `KEEP_SYMBOL=1` to keep the symbol table.

## From a release

Release archives are published per tag for linux and darwin on amd64 and arm64:

```bash
curl -fsSL -o eru-agent.tar.gz \
  https://github.com/projecteru2/agent/releases/latest/download/agent_Linux_x86_64.tar.gz
tar xzf eru-agent.tar.gz
install -m 0755 eru-agent /usr/bin/eru-agent
```

## Container image

Multi-arch images are published to Docker Hub and ghcr on every tag, and a `sha`-tagged debug image on every push to master.

```bash
docker run -d --name eru-agent --net host --privileged --restart always \
  --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:ro \
  -v /sys:/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc/:/hostProc/ \
  -v /etc/eru:/etc/eru \
  projecteru2/agent /usr/bin/eru-agent
```

The image sets `AGENT_IN_DOCKER=1`. With that variable set the agent reads host process state from `/hostProc` instead of `/proc`, which is why `/proc` has to be bind mounted. `--cgroupns=host` and the `/sys/fs/cgroup` mount are what let it read a workload's cgroup for metrics; without them every workload reports none. `--net host` is required so the agent can reach the workload ports it health checks.

## systemd

`systemd/eru-agent.service` in the repository is the unit used in production:

```bash
install -m 0644 systemd/eru-agent.service /usr/lib/systemd/system/
install -m 0644 agent.yaml.sample /etc/eru/agent.yaml
systemctl daemon-reload
systemctl enable --now eru-agent
```

The unit sets `RestartKillSignal=SIGUSR1`. That matters: on `SIGUSR1` the agent exits **without** removing its node status from core, so a restart does not make the node flap out of the cluster. On `SIGINT`, `SIGTERM` and `SIGQUIT` it removes the node status first, which is what you want when taking a node out of service.

If your systemd is too old for `RestartKillSignal=`, send `SIGUSR1` yourself before `systemctl start eru-agent`.

## Configuration file

Copy `agent.yaml.sample` to `/etc/eru/agent.yaml` and edit it. The default path is `/etc/eru/agent.yaml`; override it with `--config` or `ERU_AGENT_CONFIG_PATH`. See [configuration](configuration.md).
