# Metrics

The agent exports per-workload metrics for the Docker runtime. Yavirt guests are not sampled.

Collection starts when the agent attaches to a running workload and stops when that workload dies, at which point its gauges are unregistered and the workload disappears from `/metrics`.

## Endpoint

`GET /metrics` on `api.addr` serves the standard Prometheus text format. Set `api.addr` to an address your Prometheus can reach; leaving it empty disables the HTTP server entirely.

Sampling happens every `metrics.step` seconds. Rates are computed over that window, so a `metrics.step` of `30` means the `_per_second` gauges are 30 second averages.

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
| `mem_usage` | bytes in use |
| `mem_max_usage` | peak bytes in use |
| `mem_rss` | resident bytes |
| `mem_percent` | `mem_usage` over the workload's memory limit |
| `mem_rss_percent` | `mem_rss` over the workload's memory limit |

The two percentages are only exported when the workload has a memory limit.

### Network

Per-second rates, with an extra `nic` label naming the interface.

`bytes_send`, `bytes_recv`, `packets_send`, `packets_recv`, `err_in`, `err_out`, `drop_in`, `drop_out`

Counters are read from the workload's network namespace, which is why the container image needs `/proc` bind mounted at `/hostProc`.

### Block IO

Per device, with an extra `dev` label holding the device path resolved from its major and minor number.

| Gauge | Meaning |
|---|---|
| `io_service_bytes_read`, `io_service_bytes_write` | cumulative bytes |
| `io_serviced_read`, `io_serviced_write` | cumulative operations |
| `io_service_bytes_read_per_second`, `io_service_bytes_write_per_second` | byte rates |
| `io_serviced_read_per_second`, `io_serviced_write_per_second` | operation rates |

Device path resolution walks `/dev` looking for the device node with the matching `rdev`, so block io metrics are a Linux-only feature.

## statsd

Setting `metrics.transfers` additionally pushes every sampled value to statsd over UDP. Each workload is pinned to one transfer by hashing its id, so several statsd endpoints share the load.

The statsd key is `ERU.<appname>.<entrypoint>.<hostname>.<short container id>.<metric>`, with dots in the hostname replaced by dashes. The metric suffix matches the Prometheus gauge name, and the nic and dev variants are prefixed with the interface or device instead of being labelled.

The connection is UDP only, so a statsd endpoint that goes away is never reconnected and its samples are simply lost.
