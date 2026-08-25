# agent

Eru's per-node agent. It watches the workloads a node runs — Docker containers and yavirt virtual machines — health checks them, forwards their logs, exports their metrics and reports node and workload status back to [eru-core](https://github.com/projecteru2/core).

**Documentation: [projecteru2.github.io/agent](https://projecteru2.github.io/agent/)** (source in [`docs/`](docs/))

![lint](https://github.com/projecteru2/agent/workflows/lint/badge.svg)
![test](https://github.com/projecteru2/agent/workflows/test/badge.svg)

## Highlights

- **Node heartbeat** — reports the node alive to core on an interval, with a ttl three times the interval so one lost report does not evict the node; a clean shutdown removes the status, a `SIGUSR1` restart keeps it.
- **Workload health checks** — TCP and HTTP probes declared per workload, run on every runtime event and on a periodic sweep, with the result published through core.
- **Log forwarding** — attaches to every running workload and ships each line as a JSON record to `tcp://`, `udp://` or `journal://` targets, sharded over several targets by workload id.
- **Live log tailing** — `GET /log/?app=<name>` streams the logs of one application straight off the node.
- **Prometheus metrics** — per-workload cpu, memory, per-nic network and per-device block io gauges on `/metrics`, optionally pushed to statsd as well.
- **Two runtimes** — Docker, plus [yavirt](https://github.com/projecteru2/yavirt) for virtual machines, kept for clusters that still run guests now that yavirt is archived; a mock runtime and store cover development.

## Quick start

```bash
make build
cp agent.yaml.sample /etc/eru/agent.yaml   # edit core, docker and log settings
./eru-agent --config /etc/eru/agent.yaml
```

Or from the published image, on a node that runs Docker:

```bash
docker run -d --name eru-agent --net host --privileged --restart always \
  -v /sys:/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
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
