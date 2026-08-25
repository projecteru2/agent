# Metrics

The agent exports per-workload metrics for every source that yields a cgroup directory. Today that is the Docker source; yavirt guests are not sampled.

Collection starts when the agent first sees a workload running and stops when that workload dies, at which point its gauges are unregistered and the workload disappears from `/metrics`.

Every sample is read straight off the node — cgroup v2 files plus the network counters of the workload's namespace. A tick makes no call to any daemon, so the cost is the same whether the node runs one workload or a hundred daemons' worth. A **unified cgroup v2 hierarchy is required**; on a cgroup v1 node the agent warns and exports nothing per workload.

## Endpoint

`GET /metrics` on `api.addr` serves the standard Prometheus text format. Set `api.addr` to an address your Prometheus can reach; leaving it empty disables the HTTP server entirely.

Sampling happens every `metrics.step` seconds. Rates are computed over that window, so a `metrics.step` of `30` means the `_per_second` gauges are 30 second averages.

The node-wide cpu split that the two `cpu_host_*_usage` ratios divide by is read from `/proc/stat` at most once a second and shared by every workload, instead of once per workload per tick.

## Gauges

Every gauge carries the same constant labels: `containerID`, `hostname`, `appname`, `entrypoint`, `orchestrator` and `labels` — the last being the workload's own labels, minus eru's internal ones, flattened to `k=v,k=v`.

### CPU

Two views of the same cgroup counters. The host view divides by what the node has, the container view divides by what the workload was allowed.

| Gauge | Meaning |
|---|---|
| `cpu_host_usage` | cpu seconds used over the node's total cpu seconds in the window |
| `cpu_host_sys_usage` | share of the node's system time attributable to this workload |
| `cpu_host_user_usage` | share of the node's user time attributable to this workload |
| `cpu_container_usage` | cpu seconds used over the cpu seconds the quota allows |
| `cpu_container_sys_usage` | fraction of this workload's cpu time spent in the kernel |
| `cpu_container_user_usage` | fraction of this workload's cpu time spent in userspace |

### Memory

| Gauge | Meaning |
|---|---|
| `mem_usage` | bytes in use, from `memory.current` |
| `mem_max_usage` | peak bytes in use, from `memory.peak` |
| `mem_rss` | resident bytes, from `memory.stat` `anon` |
| `mem_percent` | `mem_usage` over the workload's memory ceiling |
| `mem_rss_percent` | `mem_rss` over the workload's memory ceiling |

`memory.peak` needs Linux 5.19. On an older kernel `mem_max_usage` is not registered at all for that workload, so it is absent from `/metrics` rather than reported as a constant `0`.

The memory ceiling is the workload's own `memory.max`, or the node's total memory when that is `max`. The two percentages are exported whenever a ceiling is known, which — since the node total is always known on Linux — means always; on a host where the agent cannot read `/proc/meminfo` an unlimited workload has no ceiling and reports neither.

### Network

Per-second rates, with an extra `nic` label naming the interface.

`bytes_send`, `bytes_recv`, `packets_send`, `packets_recv`, `err_in`, `err_out`, `drop_in`, `drop_out`

Counters are read from `/proc/<pid>/net/dev` of the workload's network namespace, which is why the container image needs `/proc` bind mounted at `/hostProc`. A workload the source reports on a host interface instead is read from `/sys/class/net/<iface>/statistics/` — one interface, no namespace.

### Block IO

Per device, with an extra `dev` label holding the device path resolved from its major and minor number.

| Gauge | Meaning |
|---|---|
| `io_service_bytes_read`, `io_service_bytes_write` | cumulative bytes |
| `io_serviced_read`, `io_serviced_write` | cumulative operations |
| `io_service_bytes_read_per_second`, `io_service_bytes_write_per_second` | byte rates |
| `io_serviced_read_per_second`, `io_serviced_write_per_second` | operation rates |

The counters come from the workload's `io.stat`. Device path resolution walks `/dev` looking for the device node with the matching `rdev`, so block io metrics are a Linux-only feature; the result is cached per major:minor, so a tick costs a map lookup and not a directory scan.

## statsd

Setting `metrics.transfers` additionally pushes every sampled value to statsd over UDP. Each workload is pinned to one transfer by hashing its id, so several statsd endpoints share the load.

The statsd key is `ERU.<appname>.<entrypoint>.<hostname>.<short container id>.<metric>`, with dots in the hostname replaced by dashes. The `nic` and `dev` variants carry the interface or device as a key prefix instead of a label, so `bytes_send` on `eth0` becomes `<prefix>.eth0.bytes.sent`.

The metric part of the key matches the Prometheus gauge name for every gauge except the eight network ones, which keep their historical dotted spelling:

| Gauge | statsd suffix |
|---|---|
| `bytes_send`, `bytes_recv` | `<nic>.bytes.sent`, `<nic>.bytes.recv` |
| `packets_send`, `packets_recv` | `<nic>.packets.sent`, `<nic>.packets.recv` |
| `err_in`, `err_out` | `<nic>.err.in`, `<nic>.err.out` |
| `drop_in`, `drop_out` | `<nic>.drop.in`, `<nic>.drop.out` |

The connection is UDP only, so a statsd endpoint that goes away is never reconnected and its samples are simply lost.
