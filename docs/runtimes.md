# Runtimes

`runtimes` lists what a node hosts. Each one implements the same `Source` interface — list the workloads, stream their events, answer whether its daemon is alive — but they do not all yield every fact a collector can use.

| Capability | `docker` | `systemd` | `mocks` |
|---|---|---|---|
| List workloads | yes | yes | yes |
| Event stream | daemon events | inotify + D-Bus | scripted |
| Daemon liveness ping | yes | yes | scripted |
| Streams its own output (attach) | yes | no, journald | yes |
| Cgroup path, so per-workload metrics | yes | yes | no |
| Health check address | yes | yes | scripted |

`mocks` needs no runtime at all and is what the test suite runs against; pair it with `store: mocks` to bring the agent up with no core either.

A node may list several runtimes. The agent then reports the union of their workloads and merges their event streams; one runtime failing tears the subscription down and the agent resubscribes to all of them. The node heartbeat needs every listed runtime alive, so a node whose Docker died stops looking alive even if its systemd units are fine.

A source that does not attach has its workloads' logs read from the journal instead; see [architecture](architecture.md). Health checks and metrics are not a source's business either: the source yields the workload's probe address, cgroup directory and netns pid, and the collectors do the rest, identically for every runtime.

## Docker

The agent talks to the local Docker API at `docker.endpoint`, negotiating the API version with the daemon on its first call. The client supports API 1.40 and up.

**Which containers it manages.** Every list and every event subscription is filtered by the eru mark label, so containers that eru did not create are invisible to the agent. `check_only_mine: true` narrows that further to containers belonging to this node. There are two ways to express "belonging":

- default — the agent inspects the container and compares its `ERU_NODE_NAME` environment variable to its own hostname
- `ERU_AGENT_EXPERIMENTAL_FILTER=label` — the agent adds `eru.nodename` and `eru.coreid` to the label filter, so the daemon does the filtering and no inspect is needed

The label path is the faster one and is the direction this is heading; it requires that core labelled the containers when it created them.

**Workload identity.** A container name must be the three part eru form `app_entrypoint_ident`. Containers whose name does not parse are skipped. Process pods carry the three parts as separate fields in their meta file, so they are not subject to this.

**Networks.** The agent reports the first network it finds on a running container. A container on the host network reports the node ip and health checks against `127.0.0.1`; any other network reports the container's own address and health checks against that.

**Resources.** The agent no longer asks Docker what a container was allowed: the metrics sampler reads `cpu.max` and `memory.max` out of the container's own cgroup on every tick. cpu is `quota / period`, falling back to the host cpu count when the quota is `max`; memory falls back to the node total when the limit is `max`. The cgroup directory comes from `/proc/<pid>/cgroup`, so it is found whatever cgroup driver the daemon uses — but it must be a **unified cgroup v2 hierarchy**. On a cgroup v1 node the agent warns and reports no metrics.

## Systemd

Process pods are transient `systemd-run` units named `eru-<workload id>.service`, created by core over ssh. There is no daemon holding their metadata, so core writes it next to the workload as `<meta_dir>/<workload id>.json` in the same ssh session, and deletes it when it removes the workload.

**Discovery.** The agent watches `meta_dir` with inotify. A file appearing is a new workload, its removal a gone one. The directory is on tmpfs, so it empties on reboot along with the transient units; an agent that starts before it exists reports no process workloads and picks them up when core creates the first one.

**Running.** A D-Bus subscription on the system bus turns every `ActiveState` change of an `eru-*.service` into a start or a die. Listing costs one `ListUnitsByPatterns` call for the whole node, not one call per workload.

**Networks.** A process pod on the host network has no counters of its own, so the agent reports none for it and health checks it against `127.0.0.1`. A pod with its own CNI address is health checked against that address, and its network counters come from the namespace of the unit's `MainPID`.

**Logs.** Transient units write to the journal natively; see [architecture](architecture.md) for the reader.

## Health checks

A workload declares its health check in the `ERU_META` label that core writes when it creates the workload. The agent reads two probe kinds from it:

- **TCP** — a list of ports. Every port must accept a connection within `healthcheck.timeout`.
- **HTTP** — one port, one path and an expected status code. The agent issues `GET http://<address>:<port><path>`. When the expected code is `0` any status from 200 to 499 counts as healthy; otherwise the status must match exactly.

Both probes must pass for the workload to be healthy. A workload that is running and declares no health check is always healthy; a workload that is not running is never healthy.

For containers the probe address is the container's own address, or `127.0.0.1` for host networking. For yavirt guests it is the first ip the guest owns.
