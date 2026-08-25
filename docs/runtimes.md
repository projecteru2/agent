# Runtimes

`runtimes` lists what a node hosts. Each one implements the same `Source` interface — list the workloads, stream their events, answer whether its daemon is alive — but they do not all yield every fact a collector can use.

| Capability | `containerd` | `systemd` | `mocks` |
|---|---|---|---|
| List workloads | yes | yes | yes |
| Event stream | daemon events | inotify + D-Bus | scripted |
| Daemon liveness ping | yes | yes | scripted |
| Cgroup path, so per-workload metrics | yes | yes | no |
| Health check address | yes | yes | scripted |

`mocks` needs no runtime at all and is what the test suite runs against; pair it with `store: mocks` to bring the agent up with no core either.

A node may list several runtimes. The agent then reports the union of their workloads and merges their event streams; one runtime failing tears the subscription down and the agent resubscribes to all of them. The node heartbeat needs every listed runtime alive, so a node whose containerd died stops looking alive even if its systemd units are fine.

Every source's logs are read from the node's journal; see [architecture](architecture.md). Health checks and metrics are not a source's business either: the source yields the workload's probe address, cgroup directory and netns pid, and the collectors do the rest, identically for every runtime.

## Containerd

The agent talks to the local containerd over `runtimes.containerd.socket`, in the namespace `runtimes.containerd.namespace`. There is no daemon in front of it: eru's core creates the container, the task and the OCI hooks itself.

**Which containers it manages.** Lists are filtered by the eru mark label, so containers eru did not create are invisible. `check_only_mine: true` narrows that to this node, either by comparing the container's `ERU_NODE_NAME` to the hostname or, with `ERU_AGENT_EXPERIMENTAL_FILTER=label`, by adding `eru.nodename` and `eru.coreid` to the daemon-side filter.

**Workload identity.** A containerd container id is the eru workload name, the three part `app_entrypoint_ident` form. Containers whose id does not parse are skipped. Pod and node names come from the `ERU_POD` and `ERU_NODE_NAME` variables of the runtime spec.

**Events.** One subscription to the events service carries `/tasks/start`, `/tasks/exit`, `/containers/delete` and `/containers/update`. An exec process exiting is not the workload exiting, so a task exit only counts when its process id is the container id. A container update is a start: it is how the addresses the OCI hook wrote back reach the agent.

**Networks.** Core has no access to the node's network namespaces, so CNI runs on the node: core writes `eru-agent oci-hook --network <name>` into the container's OCI spec as a `createRuntime` hook and again as a `poststop` hook. The hook reads the OCI state from stdin — a container with a live pid is the attach, the `stopped` state runc reports at poststop the detach — runs CNI against `/proc/<pid>/ns/net`, and writes the resulting IPv4 back as the container label `eru.network.<name>`. The agent reads the address from that label instead of entering the namespace.

A container whose spec carries no network namespace of its own shares the node's network. It has no CNI label, so the agent reports `{"host": <node ip>}` for it — the same shape core's process pods report from their meta file — and health checks it against `127.0.0.1`. The node ip is the host of the node's endpoint, read from the node record the agent looks up at startup.

**Metrics.** The cgroup directory comes from `/proc/<pid>/cgroup` of the task's pid, so it is found whatever cgroup driver containerd uses, and it must be a unified cgroup v2 hierarchy.

**Logs.** containerd keeps no logs. Core creates every task with `cio.LogURI` pointing at `eru-agent log-shim`, which writes each line to the journal under `SYSLOG_IDENTIFIER=eru` with the container id in `ERU_ID`; the agent's journal reader picks them up from there. See [architecture](architecture.md).

## Systemd

Process pods are transient `systemd-run` units named `eru-<workload id>.service`, created by core over ssh. There is no daemon holding their metadata, so core writes it next to the workload as `<meta_dir>/<workload id>.json` in the same ssh session, and deletes it when it removes the workload.

**Discovery.** The agent watches `meta_dir` with inotify. A file appearing is a new workload, its removal a gone one. The directory is on tmpfs, so it empties on reboot along with the transient units; an agent that starts before it exists reports no process workloads and picks them up when core creates the first one.

**Running.** A D-Bus subscription on the system bus turns every `ActiveState` change of a workload unit into a start or a die. Listing costs one `ListUnitsByPatterns` call for the whole node, not one call per workload.

`PropertiesChanged` fires for whatever property moved, not only for `ActiveState`, and the relist that follows a fallen-behind subscription replays every unit. The source therefore remembers what it last said about each unit and stays quiet until that changes, so a unit that simply keeps running produces no events at all. `failed` and `inactive` are one death, not two. The manager is idempotent to match: starting a workload that is already being sampled or forwarded does nothing, so neither path can restart a healthy workload's collector.

A unit counts as a workload only when it is named `eru-<workload id>.service` with the id 32 lowercase hex characters. The bus can match a glob only, so `eru-agent.service` on every node and `eru-core.service` on a core host come back from the same query and are dropped by name — otherwise the agent would report itself as a workload and chase a meta file that does not exist. A meta file whose `id` is not a workload id is rejected for the same reason, so the directory and the units cannot disagree.

**Networks.** A process pod on the host network has no counters of its own, so the agent reports none for it and health checks it against `127.0.0.1`. A pod with its own CNI address is health checked against that address, and its network counters come from the namespace of the unit's `MainPID`.

**Logs.** Transient units write to the journal natively. The agent matches them on one term, `SYSLOG_IDENTIFIER=eru`, so core starts every unit with `-p SyslogIdentifier=eru`; a unit without it is invisible to log forwarding, though its status, health checks and metrics are unaffected. See [architecture](architecture.md) for the reader.

## Health checks

A workload declares its health check in the `ERU_META` label that core writes when it creates the workload. The agent reads two probe kinds from it:

- **TCP** — a list of ports. Every port must accept a connection within `healthcheck.timeout`.
- **HTTP** — one port, one path and an expected status code. The agent issues `GET http://<address>:<port><path>`. When the expected code is `0` any status from 200 to 499 counts as healthy; otherwise the status must match exactly.

Both probes must pass for the workload to be healthy. A workload that is running and declares no health check is always healthy; a workload that is not running is never healthy.

The probe address is the workload's own CNI address, or `127.0.0.1` when it shares the host network.
