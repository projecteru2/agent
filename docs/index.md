# eru-agent

`eru-agent` runs on every Eru node. It watches the workloads the node runs, health checks them, forwards their logs, exports their metrics, and reports node and workload status back to `eru-core`. Core never talks to the node directly; the agent is the node's voice.

```
                       +-----------------------------------------------+
                       |                   eru-core                    |
                       +-----------------------------------------------+
                            ^ node status        ^ workload status
                            | (ttl heartbeat)    | (health, networks)
                            |                    |
   +------------------------+--------------------+-----------------------+
   |                             eru-agent                               |
   |                                                                     |
   |   node manager            workload manager             http api     |
   |   - heartbeat             - initial load               - /version/  |
   |   - remove on exit        - runtime event watch        - /profile/  |
   |                           - periodic health sweep      - /log/      |
   |                           - attach and forward logs    - /metrics   |
   |                           - collect metrics            - /debug/    |
   +---------------------------------+-----------------------------------+
                                     |
                    +----------------+----------------+
                    |                                 |
             docker runtime                    yavirt runtime
       (containers: attach, events,      (guests: events, status,
        stats, health checks)             health checks)
```

## Guides

- [Installation](installation.md) — building, the container image, the systemd unit
- [Configuration](configuration.md) — every key in `agent.yaml`, the flags and the environment variables
- [Architecture](architecture.md) — what the two managers do and how status reaches core
- [Runtimes](runtimes.md) — what Docker and yavirt each support, and how health checks are declared
- [Metrics](metrics.md) — the Prometheus gauges, their labels, and the statsd sink

## Source

- Repository: [projecteru2/agent](https://github.com/projecteru2/agent)
- License: MIT
