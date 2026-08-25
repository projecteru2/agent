# Runtimes

`runtime` sets which source the agent drives. All three implement the same `Source` interface — list the workloads, stream their events, answer whether the daemon is alive — but they do not all yield every fact a collector can use.

| Capability | `docker` | `yavirt` | `mocks` |
|---|---|---|---|
| List workloads | yes | yes | yes |
| Event stream | yes | yes | scripted |
| Daemon liveness ping | yes | yes | scripted |
| Attach and forward logs | yes | no | yes |
| Cgroup path, so per-workload metrics | yes | no | no |
| Health check address | yes | yes | scripted |

`mocks` needs neither Docker nor yavirt and is what the test suite runs against; pair it with `store: mocks` to bring the agent up with no core at all.

Health checks and metrics are not a source's business: the source yields the workload's probe address, cgroup directory and netns pid, and the collectors do the rest, identically for every runtime.

## Docker

The agent talks to the local Docker API at `docker.endpoint`, negotiating the API version with the daemon on its first call. The client supports API 1.40 and up.

**Which containers it manages.** Every list and every event subscription is filtered by the eru mark label, so containers that eru did not create are invisible to the agent. `check_only_mine: true` narrows that further to containers belonging to this node. There are two ways to express "belonging":

- default — the agent inspects the container and compares its `ERU_NODE_NAME` environment variable to its own hostname
- `ERU_AGENT_EXPERIMENTAL_FILTER=label` — the agent adds `eru.nodename` and `eru.coreid` to the label filter, so the daemon does the filtering and no inspect is needed

The label path is the faster one and is the direction this is heading; it requires that core labelled the containers when it created them.

**Workload identity.** A container name must be the three part eru form `app_entrypoint_ident`. Containers whose name does not parse are skipped.

**Networks.** The agent reports the first network it finds on a running container. A container on the host network reports the node ip and health checks against `127.0.0.1`; any other network reports the container's own address and health checks against that.

**Resources.** The agent no longer asks Docker what a container was allowed: the metrics sampler reads `cpu.max` and `memory.max` out of the container's own cgroup on every tick. cpu is `quota / period`, falling back to the host cpu count when the quota is `max`; memory falls back to the node total when the limit is `max`. The cgroup directory comes from `/proc/<pid>/cgroup`, so it is found whatever cgroup driver the daemon uses — but it must be a **unified cgroup v2 hierarchy**. On a cgroup v1 node the agent warns and reports no metrics.

## Yavirt

[yavirt](https://github.com/projecteru2/yavirt) and its client library are archived upstream. The runtime stays in the agent for clusters that still run guests, and gets no new features.

The agent talks to yavirt over gRPC at `yavirt.endpoint`.

Guests are filtered by the same eru mark label. `check_only_mine: true` compares the guest's hostname to the agent's. `skip_guest_report_regexps` gives a second, explicit exclusion: any guest id matching one of the expressions is treated as not ours.

Yavirt guests have no log attach and no metrics: the source does not implement `source.Attacher` and yields no cgroup path, so the manager skips both. Health checks and status reporting work exactly as they do for containers.

## Health checks

A workload declares its health check in the `ERU_META` label that core writes when it creates the workload. The agent reads two probe kinds from it:

- **TCP** — a list of ports. Every port must accept a connection within `healthcheck.timeout`.
- **HTTP** — one port, one path and an expected status code. The agent issues `GET http://<address>:<port><path>`. When the expected code is `0` any status from 200 to 499 counts as healthy; otherwise the status must match exactly.

Both probes must pass for the workload to be healthy. A workload that is running and declares no health check is always healthy; a workload that is not running is never healthy.

For containers the probe address is the container's own address, or `127.0.0.1` for host networking. For yavirt guests it is the first ip the guest owns.
