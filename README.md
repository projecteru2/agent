# agent

Eru's per-node agent. It watches the workloads a node runs — containerd containers, systemd process pods and cocoon VMs — health checks them, forwards their logs, exports their metrics and reports node and workload status back to [eru-core](https://github.com/projecteru2/core).

**Documentation: [projecteru2.github.io/agent](https://projecteru2.github.io/agent/)** (source in [`docs/`](docs/))

![lint](https://github.com/projecteru2/agent/workflows/lint/badge.svg)
![test](https://github.com/projecteru2/agent/workflows/test/badge.svg)

## Highlights

- **Node heartbeat** — reports the node alive to core on an interval, with a ttl three times the interval so one lost report does not evict the node; a clean shutdown removes the status, a `SIGUSR1` restart keeps it.
- **Workload health checks** — TCP and HTTP probes declared per workload, run on every runtime event and on a periodic sweep, with the result published through core.
- **Log forwarding** — ships each line as a JSON record to `tcp://`, `udp://` or `journal://` targets, sharded over several targets by workload id, read from the node's journal with a persisted cursor, or tailed off a VM's console file. Container output reaches the journal through `eru-agent log-shim`, containerd's binary logger.
- **Live log tailing** — `GET /log/?app=<name>` streams the logs of one application straight off the node.
- **Prometheus metrics** — per-workload cpu, memory, per-nic network and per-device block io gauges on `/metrics`, optionally pushed to statsd as well. Sampled straight from cgroup v2 files, so a tick makes no call to any daemon.
- **Runtimes per node** — a node declares the runtimes it hosts under a required `runtimes:` section: containerd containers, systemd process pods, cocoon VMs, or several at once; the heartbeat needs every one of them alive. The old `runtime:` and `docker:` keys are gone, with no compatibility shim. A mock runtime and store cover development.
- **Node-side helper modes** — the same binary is containerd's task logger (`eru-agent log-shim`) and its CNI hook (`eru-agent oci-hook`), so a container node needs no eru daemon beyond the agent.

## Quick start

```bash
make build
cp agent.yaml.sample /etc/eru/agent.yaml   # edit core, runtimes and log settings
./eru-agent --config /etc/eru/agent.yaml
```

Or from the published image:

```bash
docker run -d --name eru-agent --net host --privileged --restart always \
  --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:ro \
  -v /sys:/sys:ro \
  -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
  -v /proc/:/hostProc/ \
  -v /etc/eru:/etc/eru \
  projecteru2/agent /usr/bin/eru-agent
```

See [installation](docs/installation.md) for the systemd unit and [configuration](docs/configuration.md) for every option.

## Related projects

- [core](https://github.com/projecteru2/core) — the scheduler this agent reports to
- [cli](https://github.com/projecteru2/cli) — command line client for core
- [resource-extend](https://github.com/projecteru2/resource-extend) — extra resource plugins for core
- [quickstart](https://github.com/projecteru2/quickstart) — Ansible playbook that stands up a cluster, this agent included
- [footstone](https://github.com/projecteru2/footstone) — the build and lambda images Eru runs code in

## Development

```bash
make build    # build the eru-agent binary
make test     # go vet plus tests with the race detector
make lint     # golangci-lint on linux and darwin
make fmt      # gofumpt + goimports
make mock     # regenerate the testify mocks
make all      # deps, fmt, lint, test, build
```

## License

This project is licensed under the MIT License. See [`LICENSE`](./LICENSE).
