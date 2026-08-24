# Runtimes

`runtime` sets which backend the agent drives. All three implement the same `Runtime` interface, but they do not all implement every operation.

| Capability | `docker` | `yavirt` | `mocks` |
|---|---|---|---|
| List workloads | yes | yes | yes |
| Event stream | yes | yes | scripted |
| Status and health check | yes | yes | scripted |
| Attach and forward logs | yes | no | yes |
| Per-workload metrics | yes | no | no |
| Daemon liveness ping | yes | yes | scripted |

`mocks` needs neither Docker nor yavirt and is what the test suite runs against; pair it with `store: mocks` to bring the agent up with no core at all.

## Docker

The agent talks to the local Docker API at `docker.endpoint` using API version 1.35, so it works against any reasonably modern daemon.

**Which containers it manages.** Every list and every event subscription is filtered by the eru mark label, so containers that eru did not create are invisible to the agent. `check_only_mine: true` narrows that further to containers belonging to this node. There are two ways to express "belonging":

- default — the agent inspects the container and compares its `ERU_NODE_NAME` environment variable to its own hostname
- `ERU_AGENT_EXPERIMENTAL_FILTER=label` — the agent adds `eru.nodename` and `eru.coreid` to the label filter, so the daemon does the filtering and no inspect is needed

The label path is the faster one and is the direction this is heading; it requires that core labelled the containers when it created them.

**Workload identity.** A container name must be the three part eru form `app_entrypoint_ident`. Containers whose name does not parse are skipped.

**Networks.** The agent reports the first network it finds on a running container. A container on the host network reports the node ip and health checks against `127.0.0.1`; any other network reports the container's own address and health checks against that.

**Resources.** cpu is `CpuQuota / CpuPeriod` when both are set, otherwise the host cpu count. Memory is the larger of the container's memory limit and reservation, falling back to host total memory when neither is set.

## Yavirt

The agent talks to yavirt over gRPC at `yavirt.endpoint`.

Guests are filtered by the same eru mark label. `check_only_mine: true` compares the guest's hostname to the agent's. `skip_guest_report_regexps` gives a second, explicit exclusion: any guest id matching one of the expressions is treated as not ours.

Yavirt guests have no log attach and no metrics collection: `AttachWorkload` and `GetWorkloadName` return "not implemented" and the workload manager logs that at debug level and moves on. Health checks and status reporting work exactly as they do for containers.

## Health checks

A workload declares its health check in the `ERU_META` label that core writes when it creates the workload. The agent reads two probe kinds from it:

- **TCP** — a list of ports. Every port must accept a connection within `healthcheck.timeout`.
- **HTTP** — one port, one path and an expected status code. The agent issues `GET http://<address>:<port><path>`. When the expected code is `0` any status from 200 to 499 counts as healthy; otherwise the status must match exactly.

Both probes must pass for the workload to be healthy. A workload that is running and declares no health check is always healthy; a workload that is not running is never healthy.

For containers the probe address is the container's own address, or `127.0.0.1` for host networking. For yavirt guests every ip the guest owns is probed.
