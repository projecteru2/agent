# Architecture

The agent is one process with two independent managers and an HTTP server. `main` calls `manager.NewClients` once — it dials the core store and every runtime the node hosts — and hands the same pair to both managers, so there is one connection pool to core and one connection to the runtime daemon no matter how many managers want them. If either manager's `Run` returns an error the process cancels its context and exits.

Everything runtime-specific lives in a **source**: it lists the workloads the node runs, streams their state changes and answers whether its daemon is alive. Everything else — the metrics sampler, the health prober — is a runtime-agnostic **collector** that reads Linux files. A source hands a collector a `source.Workload`: the id, the metadata core wrote at create time, the workload's cgroup directory, the pid whose network namespace to read (or a host interface), and where its log lines are. No collector makes an IPC call.

The binary is also the two helper modes containerd runs on the node: `eru-agent log-shim` as a task's logger and `eru-agent oci-hook` as its CNI hook. Neither reads the agent's configuration nor talks to core.

## Packages

| Package | Responsibility |
|---|---|
| `manager` | Builds the store and source both managers share |
| `manager/node` | Node status heartbeat and shutdown |
| `manager/workload` | Workload discovery, health checks, log forwarding and broadcast |
| `source` | The `Source` interface every runtime implements, the `Workload` it yields, and the dedup that keeps a repeated state from becoming a second event |
| `source/containerd`, `source/systemd`, `source/cocoon` | The runtime backends, plus `source.Multi` for a node that hosts several |
| `source/meta` | The metadata file core writes over ssh, shared by the runtimes it describes |
| `collector` | Runtime-agnostic hot paths: cgroup v2 metrics, network counters, health probes, journal reader and console reader |
| `logshim` | The `eru-agent log-shim` mode: containerd's binary logger, one process per task |
| `ocihook` | The `eru-agent oci-hook` mode: cni attach and detach from the container's own oci spec |
| `store` | The `Store` interface the managers report through |
| `store/core` | gRPC client pool talking to `eru-core` |
| `logs` | Log record encoders and the reconnecting forwarder |
| `api` | The HTTP server |
| `types`, `utils`, `common`, `version` | Leaf packages |

## Node manager

`manager/node` reports this node alive. Every `heartbeat_interval` seconds it:

1. Pings every runtime the node is configured with. If any of them is unreachable the report is skipped, so a node whose containerd died stops looking alive and core expires it.
2. Calls `SetNodeStatus` with a ttl of three times the interval, retrying three times with exponential backoff.

The ttl outlives the interval on purpose: a single lost or slow report must not evict the node.

On shutdown the behaviour depends on the signal:

- `SIGINT`, `SIGTERM`, `SIGQUIT` — the agent calls `SetNodeStatus` with a ttl of `-1`, which removes the status. Core sees the node leave immediately.
- `SIGUSR1` — the agent exits without touching the status. This is the restart path, and it is why the systemd unit sets `RestartKillSignal=SIGUSR1`.

The final removal waits for any in-flight heartbeat write and prevents another report from starting, so an older heartbeat cannot restore the status after the removal. The store bounds each write itself, so the removal gets a full timeout budget even after waiting out a slow heartbeat.

## Workload manager

`manager/workload` owns everything about the workloads on the node.

**Initial load.** At startup it lists every workload carrying the eru mark, retrying until the runtime answers, then for each one fetches status, starts forwarding and sampling if it is running, and reports the status to core. Startup blocks until this sweep finishes, so core has a complete picture before the agent starts reacting to events.

**Event watch.** It subscribes to the runtime's event stream, filtered to the same set of workloads, and handles two actions:

- `start` — fetch the workload and run the same check the sweep uses: forwarding and sampling start before the health probe so no early log line is lost, and the status — healthy or not — is reported at once. An unhealthy result — or a workload the runtime cannot describe yet, its meta file ahead of its unit — arms a backoff retry task that re-checks until it becomes healthy or the attempts run out; a second `start` for the same workload cancels the previous task.
- `die` — stop local sampling and forwarding, then ask the runtime. A workload it still knows goes through the same check the sweep uses, so a die raised by mistake restarts what was just stopped and reports the true state; one it denies is reported gone by id, with the appname and entrypoint remembered from forwarding so core resolves it without a lookup. A final stop closes the window where a stale concurrent check restarted the tasks while the status write was in flight.

If the stream errors the manager waits `global_connection_timeout` and resubscribes, so a runtime daemon restart is survivable.

**Health sweep.** Every `healthcheck.interval` seconds it lists **all** runtime workloads, stopped ones included, and the workloads core still considers running on this node. It re-checks every runtime workload and stops local sampling and forwarding for stopped ones. A core-running workload missing from the listing is asked for once more by id: a runtime that still knows it gets the normal check, and only one that denies it is reported gone, so a transient inspect failure cannot bury a live workload. The sweep also reconciles the local sampling and forwarding tasks against both listings: a task whose workload is in neither is confirmed dead with the runtime and stopped, so nothing a race left behind outlives the next sweep. The next sweep therefore repairs both a delete missed while the event stream reconnects and a stale running check that finishes after a die event. A tick that finds the previous sweep still running is skipped.

**Metrics.** Each running workload gets one sampling goroutine, started when the workload is first seen running and cancelled when a die event or health sweep observes it stopped. The sampler reads the workload's cgroup v2 files and its netns counters directly, so a tick costs a handful of small file reads and no call to any daemon. See [metrics](metrics.md).

**Logs.** A workload's output reaches the agent one of two ways, and the source decides which.

- **Console** — a VM's output is its serial console, which Cloud Hypervisor serves as a unix socket for a UEFI guest and as a PTY for a direct-boot one. One goroutine per VM stays blocked reading it, forwarding every line and writing it to journald under the same fields the log shim uses, so `journalctl ERU_ID=<id>` answers for a VM exactly as it does for a container. A line is cut at the 64KB read buffer — the same bound the log shim applies — so a guest spraying an endless unterminated line cannot push a record past the journal reader's limit. A console that is not there yet, or that went away with its VM, is retried on a capped backoff; the reader picks the VM up again when it comes back. The journal reader skips what it finds under `ERU_STREAM=console`, since the reader that wrote those lines already forwarded them.
- **Journal** — every other runtime logs to journald. The agent runs one `journalctl --follow --output=json --all SYSLOG_IDENTIFIER=eru` child process for the whole node, and routes each record to a workload by its `ERU_ID` field or by the unit that emitted it. `--all` matters: without it journalctl nulls any field over 4KB, which would silently empty every long log line. A record without an `ERU_STREAM` field is a unit's own output; its journald priority decides the stream, so a unit's stderr is forwarded as stderr. One term, not two: journald ors terms with `+`, but `-u` is an option rather than a term, so `-u 'eru-*' + SYSLOG_IDENTIFIER=eru` is rejected. Everything eru runs therefore carries the same identifier — process units get `SyslogIdentifier=eru` from core's `systemd-run`, containers get it from `eru-agent log-shim`, which containerd execs once per task. One reader per node replaces one attach per workload, and a cursor persisted under `state_dir` means an agent restart resumes where it stopped instead of losing the lines in between. journald's format is only readable through libsystemd, so the system tool is the reader; the agent logs that requirement at debug level on startup.

Either way, each line becomes the same JSON record:

```json
{
  "id": "3e0a...",
  "name": "myapp",
  "type": "stdout",
  "entrypoint": "web",
  "ident": "EAXPcM",
  "data": "the log line",
  "datetime": "2026-08-25 03:14:15.926535",
  "extra": {"podname": "prod", "nodename": "node-1", "coreid": "...", "networks_bridge": "10.0.0.5"}
}
```

The record goes to two places: the configured forwarder for that workload, and the in-process broadcaster that serves `/log/`. Non-utf8 bytes are escaped as `\xNN` so a binary blob on stdout cannot corrupt the stream.

**Log broadcaster.** `GET /log/?app=<name>` hijacks the connection and subscribes to every record whose `name` matches. Each reader broadcasts its lines in order, and one workload's lines come from one reader, so a subscriber sees a workload's lines in order. A subscriber whose connection breaks — detected by a read on the hijacked socket — is cancelled and dropped; one that falls more than 256 lines behind loses the excess rather than stalling the node's forwarding, counted in `log_lines_dropped_total{point="subscriber"}`.

## Status reporting

Both the event path and the sweep report through `store.Store`. The core store sends `SetWorkloadsStatus` with a ttl of `0`, which means core owns expiry — the agent does not set a workload ttl because selfmon lives in core now. To avoid re-sending an unchanged status every sweep, the core store caches the last reported status per workload for `healthcheck.cache_ttl` seconds, with per-workload jitter.

## Concurrency

Health probes for one workload are deduplicated by a per-id compare-and-swap lock. Sampling and log forwarding have one task per workload under their own locks; repeated starts leave the task in place, while a die event or stopped health result removes it. Everything else is plain goroutines; the process has no worker pool and no `init()`.
