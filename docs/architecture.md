# Architecture

The agent is one process with two independent managers and an HTTP server. Both managers ask `manager.NewClients` for the same two things — the core store and the runtime source — and the clients behind it are per-process singletons, so there is one connection pool to core and one connection to the runtime daemon no matter how many managers want them. If either manager's `Run` returns an error the process cancels its context and exits.

Everything runtime-specific lives in a **source**: it lists the workloads the node runs, streams their state changes and answers whether its daemon is alive. Everything else — the metrics sampler, the health prober — is a runtime-agnostic **collector** that reads Linux files. A source hands a collector a `source.Workload`: the id, the metadata core wrote at create time, the workload's cgroup directory, the pid whose network namespace to read (or a host interface), and where its log lines are. No collector makes an IPC call.

## Packages

| Package | Responsibility |
|---|---|
| `manager` | Builds the store and source both managers share |
| `manager/node` | Node status heartbeat and shutdown |
| `manager/workload` | Workload discovery, health checks, log attach and broadcast |
| `source` | The `Source` interface every runtime implements, and the `Workload` it yields |
| `source/docker`, `source/yavirt` | The two runtime backends |
| `collector` | Runtime-agnostic hot paths: cgroup v2 metrics, network counters, health probes |
| `store` | The `Store` interface the managers report through |
| `store/core` | gRPC client pool talking to `eru-core` |
| `logs` | Log record encoders and the reconnecting forwarder |
| `api` | The HTTP server |
| `types`, `utils`, `common`, `version` | Leaf packages |

## Node manager

`manager/node` reports this node alive. Every `heartbeat_interval` seconds it:

1. Pings the runtime daemon through the source. If the daemon is unreachable the report is skipped, so a node whose Docker died stops looking alive and core expires it.
2. Calls `SetNodeStatus` with a ttl of three times the interval, retrying three times with exponential backoff.

The ttl outlives the interval on purpose: a single lost or slow report must not evict the node.

On shutdown the behaviour depends on the signal:

- `SIGINT`, `SIGTERM`, `SIGQUIT` — the agent calls `SetNodeStatus` with a ttl of `-1`, which removes the status. Core sees the node leave immediately.
- `SIGUSR1` — the agent exits without touching the status. This is the restart path, and it is why the systemd unit sets `RestartKillSignal=SIGUSR1`.

## Workload manager

`manager/workload` owns everything about the workloads on the node.

**Initial load.** At startup it lists every workload carrying the eru mark, retrying until the runtime answers, then for each one fetches status, attaches if it is running, and reports the status to core. Startup blocks until this sweep finishes, so core has a complete picture before the agent starts reacting to events.

**Event watch.** It subscribes to the runtime's event stream, filtered to the same set of workloads, and handles two actions:

- `start` — fetch status; attach if running; if the workload is already healthy, report it, otherwise start a backoff retry task that re-checks it until it becomes healthy or the attempts run out. A second `start` for the same workload cancels the previous task.
- `die` — fetch status and report it.

If the stream errors the manager waits `global_connection_timeout` and resubscribes, so a runtime daemon restart is survivable.

**Health sweep.** Every `healthcheck.interval` seconds it lists **all** workloads, stopped ones included, and re-checks each. Listing all of them is deliberate: an event-driven check that returns late must not be the last word on a workload the die event already buried.

**Metrics.** Each running workload gets one sampling goroutine, started when the workload is first seen running and cancelled on its die event. The sampler reads the workload's cgroup v2 files and its netns counters directly, so a tick costs a handful of small file reads and no call to any daemon. See [metrics](metrics.md).

**Attach.** A source that streams a workload's output implements `source.Attacher`; for each running workload the manager opens its stdout and stderr and pumps them line by line. Each line becomes a JSON record:

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

**Log broadcaster.** `GET /log/?app=<name>` hijacks the connection and subscribes to every record whose `name` matches. Records are written to all subscribers of an app before the next record is processed, so a subscriber sees the lines in order. A subscriber whose connection breaks is cancelled and dropped.

## Status reporting

Both workload paths report through `store.Store`. The core store sends `SetWorkloadsStatus` with a ttl of `0`, which means core owns expiry — the agent does not set a workload ttl because selfmon lives in core now. To avoid re-sending an unchanged status every sweep, the core store caches the last reported status per workload for `healthcheck.cache_ttl` seconds, with per-workload jitter.

## Concurrency

Health checks for one workload are serialised by a per-id compare-and-swap lock, so an event-driven check and a sweep check cannot run against the same workload at once. Metrics sampling is deduplicated the same way: starting a sampler for a workload cancels the one already running for that id. Everything else is plain goroutines; the process has no worker pool and no `init()`.
