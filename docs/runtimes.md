# Runtimes

`runtimes` lists what a node hosts. Each one implements the same `Source` interface — list the workloads, stream their events, answer whether its daemon is alive — but they do not all yield every fact a collector can use.

| Capability | `containerd` | `systemd` | `cocoon` | `mocks` |
|---|---|---|---|---|
| List workloads | yes | yes | yes | yes |
| Event stream | daemon events | inotify + D-Bus | inotify + daemon SSE | scripted |
| Daemon liveness ping | yes | yes | when a daemon runs | scripted |
| Where its logs come from | journald | journald | serial console | scripted |
| Cgroup path, so per-workload metrics | yes | yes | yes | no |
| Health check address | yes | yes | yes | scripted |

`mocks` needs no runtime at all and is what the test suite runs against; pair it with `store: mocks` to bring the agent up with no core either.

A node may list several runtimes. The agent then reports the union of their workloads and merges their event streams; one runtime failing tears the subscription down and the agent resubscribes to all of them. The node heartbeat needs every listed runtime alive, so a node whose containerd died stops looking alive even if its systemd units are fine.

A source's logs are read from the node's journal, or off the serial console its metadata names; see [architecture](architecture.md). Health checks and metrics are not a source's business either: the source yields the workload's probe address, cgroup directory and netns pid, and the collectors do the rest, identically for every runtime.

## Containerd

The agent talks to the local containerd over `runtimes.containerd.socket`, in the namespace `runtimes.containerd.namespace`. There is no daemon in front of it: eru's core creates the container, the task and the OCI hooks itself.

**Which containers it manages.** Lists are filtered by the eru mark label, so containers eru did not create are invisible. `check_only_mine: true` narrows that to this node by the `eru.nodename` label core wrote at create time. Only where the comparison happens changes: by default the agent makes it itself on every container it lists, and with `ERU_AGENT_EXPERIMENTAL_FILTER=label` it adds `eru.nodename` and `eru.coreid` to the filter so the daemon does it instead.

**Workload identity.** A containerd container id is the eru workload name, the three part `app_entrypoint_ident` form. Containers whose id does not parse are skipped. Pod and node names come from the `ERU_POD` and `ERU_NODE_NAME` variables of the runtime spec.

**Events.** One subscription to the events service carries `/tasks/start`, `/tasks/exit`, `/containers/delete` and `/containers/update`. An exec process exiting is not the workload exiting, so a task exit only counts when its process id is the container id. A container update is a start: it is how the addresses the OCI hook wrote back reach the agent.

**Networks.** Core has no access to the node's network namespaces, so CNI runs on the node: core writes `eru-agent oci-hook --network <name>` into the container's OCI spec as a `createRuntime` hook and again as a `poststop` hook. The hook reads the OCI state from stdin — a container with a live pid is the attach, the `stopped` state runc reports at poststop the detach — runs CNI against `/proc/<pid>/ns/net`, and writes the resulting IPv4 back as the container label `eru.network.<name>`. The agent reads the address from that label instead of entering the namespace.

A container whose spec carries no network namespace of its own shares the node's network. It has no CNI label, so the agent reports `{"host": <node ip>}` for it — the same shape core's process pods report from their meta file — and health checks it against `127.0.0.1`. The node ip is the host of the node's endpoint, read from the node record the agent looks up at startup.

**Metrics.** The cgroup directory comes from `/proc/<pid>/cgroup` of the task's pid, so it is found whatever cgroup driver containerd uses, and it must be a unified cgroup v2 hierarchy.

**Logs.** containerd keeps no logs. Core creates every task with `cio.LogURI` set to `binary:///usr/local/bin/eru-agent?log-shim`, which writes each line to the journal under `SYSLOG_IDENTIFIER=eru` with the container id in `ERU_ID`; the agent's journal reader picks them up from there. See [architecture](architecture.md).

## Systemd

Process pods are transient `systemd-run` units named `eru-<workload id>.service`, created by core over ssh. There is no daemon holding their metadata, so core writes it next to the workload as `<meta_dir>/<workload id>.json` in the same ssh session, and deletes it when it removes the workload.

**Discovery.** The agent watches `meta_dir` with inotify. A file appearing is a new workload, its removal a gone one. Only the files whose `kind` is `process` are this source's; the rest belong to another runtime sharing the directory. The directory is on tmpfs, so it empties on reboot along with the transient units; an agent that starts before it exists reports no process workloads and picks them up when core creates the first one.

**Running.** A D-Bus subscription on the system bus turns every `ActiveState` change of a workload unit into a start or a die. Listing costs one `ListUnitsByPatterns` call for the whole node, not one call per workload.

`PropertiesChanged` fires for whatever property moved, not only for `ActiveState`, and every new subscription — and any that fell behind — is followed by a relist replaying every unit, so a transition during a resubscribe gap is not lost. The source therefore remembers what it last said about each unit and stays quiet until that changes, so a unit that simply keeps running produces no events at all. `failed` and `inactive` are one death, not two. The manager is idempotent to match: starting a workload that is already being sampled or forwarded does nothing, so neither path can restart a healthy workload's collector.

A unit counts as a workload only when it is named `eru-<workload id>.service` with the id 32 lowercase hex characters. The bus can match a glob only, so `eru-agent.service` on every node and `eru-core.service` on a core host come back from the same query and are dropped by name — otherwise the agent would report itself as a workload and chase a meta file that does not exist. A meta file whose `id` is not a workload id is rejected for the same reason, so the directory and the units cannot disagree.

**Networks.** A process pod on the host network has no counters of its own, so the agent reports none for it and health checks it against `127.0.0.1`. A pod with its own CNI address is health checked against that address, and its network counters come from the namespace of the unit's `MainPID`.

**Logs.** Transient units write to the journal natively. The agent matches them on one term, `SYSLOG_IDENTIFIER=eru`, so core starts every unit with `-p SyslogIdentifier=eru`; a unit without it is invisible to log forwarding, though its status, health checks and metrics are unaffected. See [architecture](architecture.md) for the reader.

## Cocoon

VM pods are Cloud Hypervisor or Firecracker guests that the `cocoon` CLI created over ssh. Like a process pod they carry no runtime metadata, so core writes `<meta_dir>/<workload id>.json` in the same ssh session and removes it when it removes the VM. Both runtimes share the directory and the inotify watch on it; a meta file names the runtime it belongs to under `kind`, so the systemd source claims `process` files and the cocoon source claims `vm` files and neither sees the other's.

**Discovery.** Same as a process pod: a file appearing is a new VM, its removal a gone one.

**Running.** `cocoon daemon` supervises the VMs on the node and serves a read-only API on `runtimes.cocoon.socket`. That socket is `root:root` mode `0660`, so on a cocoon node the agent runs as root; nothing else authenticates against it. The agent opens `GET /v1/events` once and turns every change into a start or a die; `GET /v1/vms` answers the same question for a listing. VMs are matched to workloads by the name core created them under, so a VM an operator created outside eru is ignored.

The daemon reports a state and, beside it, whether it saw the VMM alive on its last pass — and the two can lag each other in either direction. A VM the daemon saw alive counts as running whatever its record momentarily says; a record that says `running` while the daemon did not see the VMM is answered by the VM's own cgroup instead, since the daemon itself refuses to read an inconclusive probe as an exit. A record that has settled on any other state — `creating` included, so a VM whose VMM has not launched yet — is not running. The source reports a workload only when what it says about it changes, so core sees each transition once.

**Without the daemon.** It is optional. When nothing answers on the socket the agent falls back to the VM's own cgroup scope: a `cgroup.procs` with a pid in it is a running VM, read on the health tick like every other file the collectors read. Because that fallback is always there, a cocoon node is alive as long as its meta dir is readable; a daemon that is installed but not answering is logged as a warning and does not stop the node heartbeat, since every VM on the node is still running and still being reported.

**Networks.** A VM's tap lives in a network namespace of its own, not on the host, so it is invisible in `/sys/class/net`. The meta file names both the tap and the VMM's pid — the VMM runs inside that namespace — and the counters come from `/proc/<netns_pid>/net/dev` narrowed to the tap, the same mechanism a container's netns counters use. Until the VM has booted the pid is `0` and there is nothing to read, so the agent skips the host lookup entirely and reports the cgroup gauges alone. A failed cgroup read refreshes the meta file before the next sample, and the periodic health sweep also replaces a sampler whose cgroup or network namespace identity changed, so a pid core rewrites after a restart is adopted even when the runtime event was missed. The tap is the far end of the guest's link, so what it received is what the guest sent: the agent flips rx and tx for a VM, and the gauges read from the workload's side like every other runtime's. Health checks go against the CNI address in the same file.

**Logs.** A VM writes to its serial console, not to the journal — Linux guests their console output, Windows guests SAC. Cloud Hypervisor serves that console as a unix socket for a UEFI guest and as a PTY for a direct-boot one, and `log.console_socket` in the meta file is whichever one the VM got; the file cocoon keeps under its log dir is the VMM's own log, never guest output, so the agent does not read it. One goroutine per VM stays blocked on the console, forwarding each line and writing it to journald under `SYSLOG_IDENTIFIER=eru` with `ERU_ID`, `ERU_NAME` and `ERU_STREAM=console`, so core reads a VM's history back over ssh the same way it reads a container's. A console that is missing or has gone away with its VM is retried on a capped backoff, and the path is read back out of the meta file on every attempt rather than remembered from create time: Cloud Hypervisor hands a direct-boot guest a fresh PTY on every start, and core rewrites the meta file to match. A socket hands over its backlog when the reader connects, but a PTY has none, so whatever a direct-boot guest wrote before the reader's first open is lost.

## Health checks

A container declares its health check in the `ERU_META` label that core writes when it creates the container; a process pod or a VM declares it in the `healthcheck` object of its meta file. The agent reads two probe kinds from either:

- **TCP** — a list of ports. Every port must accept a connection within `healthcheck.timeout`.
- **HTTP** — one port, one path and an expected status code. The agent issues `GET http://<address>:<port><path>`. When the expected code is `0` any status from 200 to 499 counts as healthy; otherwise the status must match exactly.

Both probes must pass for the workload to be healthy. A workload that is running and declares no health check is always healthy; a workload that is not running is never healthy.

The probe address is the workload's own CNI address, or `127.0.0.1` for a process pod sharing the host network. A VM never shares the host network, so a VM without an address of its own has nothing to probe and reports unhealthy instead of borrowing the node's loopback.
