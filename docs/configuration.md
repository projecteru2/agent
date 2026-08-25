# Configuration

The agent reads a YAML file, then lets command line flags and environment variables override it. `agent.yaml.sample` in the repository is a complete annotated example.

Precedence is: flag or environment variable, then the YAML file, then the built-in default.

## Top level

| Key | Default | Meaning |
|---|---|---|
| `pid` | `/tmp/agent.pid` | File the agent writes its pid to at startup and removes on exit |
| `core` | required | Addresses of the `eru-core` gRPC endpoints; the agent load balances across them |
| `heartbeat_interval` | `60` | Seconds between node status reports; `0` disables the heartbeat entirely |
| `check_only_mine` | `false` | Ignore workloads that belong to another node (see [runtimes](runtimes.md)) |
| `store` | `grpc` | `grpc` talks to core, `mocks` uses an in-memory fake for development |
| `meta_dir` | `/run/eru/workloads` | Directory core writes workload metadata into over ssh, one `<id>.json` per workload |
| `state_dir` | `/var/lib/eru-agent` | Directory the agent keeps what it must survive a restart in, currently the journal cursor |
| `global_connection_timeout` | `5s` | Timeout for every call to core and to a runtime daemon |

## `auth`

Credentials for core, when core requires them. Omit the section when it does not.

```yaml
auth:
  username: eru
  password: secret
```

## `runtimes`

Which runtimes this node hosts. Name only the ones it actually runs: **every runtime listed here must be reachable**, otherwise the node heartbeat stops and core expires the node. The agent refuses to start with the section empty.

```yaml
runtimes:
  docker:
    endpoint: unix:///var/run/docker.sock
  systemd: {}
```

| Runtime | Keys | Liveness check |
|---|---|---|
| `docker` | `endpoint` — the local Docker API, a unix socket path or a `tcp://host:port` address; defaults to `unix:///var/run/docker.sock`. The agent negotiates the API version on its first call, so it works against any daemon from API 1.40 up. | daemon ping |
| `systemd` | none — a process pod is described by its meta file in `meta_dir`, and its unit is read over the system D-Bus | D-Bus unit listing |
| `mocks` | none — the scripted runtime the test suite runs against; pair it with `store: mocks` to bring the agent up with neither a runtime nor a core | scripted |

A node listing several runtimes reports the union of their workloads, and merges their event streams into one; a failure in any of them tears the subscription down and the agent resubscribes to all of them.

## `healthcheck`

```yaml
healthcheck:
  interval: 120
  timeout: 10
  cache_ttl: 300
```

| Key | Default | Meaning |
|---|---|---|
| `interval` | `60` | Seconds between full health sweeps over every workload on the node |
| `timeout` | `10` | Seconds a single TCP or HTTP probe may take |
| `cache_ttl` | `300` | Seconds an unchanged workload status is remembered locally so it is not re-sent to core |

The cache ttl is jittered per workload so a node does not re-report every workload in the same second.

## `log`

```yaml
log:
  forwards:
    - tcp://127.0.0.1:5144
  stdout: false
```

`forwards` is a list of targets. Supported schemes are `tcp://`, `udp://` and `journal://`; a target with any other scheme is accepted and silently discards, with a warning at startup. Each workload is pinned to one target by hashing its id, so several targets share the load without duplicating lines. A target that is down is retried every 30 seconds in the background while its lines are dropped.

Where the lines come from depends on the runtime. Docker workloads are attached to directly. Every other runtime logs to journald, and the agent runs one `journalctl --follow --output=json SYSLOG_IDENTIFIER=eru` for the whole node, resuming from the cursor it saved under `state_dir`. That path needs `journalctl` on the node, and needs every eru workload to log under the `eru` syslog identifier.

`stdout: true` additionally writes every forwarded line to the agent's own log.

## `metrics`

```yaml
metrics:
  step: 30
  transfers:
    - 127.0.0.1:8125
```

`step` is the sampling interval in seconds. `transfers` are statsd endpoints; leave it empty to export to Prometheus only. Like log forwards, a workload is pinned to one transfer by hashing its id. See [metrics](metrics.md).

## `api`

```yaml
api:
  addr: 127.0.0.1:12345
```

Address of the agent's HTTP server. Leave it empty and the agent serves no HTTP at all. The server exposes:

- `GET /version/` — the agent version as JSON
- `GET /profile/` — the live goroutine, heap and thread profile counts as JSON
- `GET /log/?app=<name>` — a chunked stream of that application's log lines
- `GET /metrics` — the Prometheus endpoint
- `/debug/pprof/` — the standard `net/http/pprof` handlers

## Flags and environment variables

Every flag has an environment variable. `--core-endpoint` and the two list flags may be repeated.

| Flag | Environment variable |
|---|---|
| `--config` | `ERU_AGENT_CONFIG_PATH` |
| `--log-level` | `ERU_AGENT_LOG_LEVEL` |
| `--store` | `ERU_AGENT_STORE` |
| `--core-endpoint` | `ERU_AGENT_CORE_ENDPOINT` |
| `--core-username` | `ERU_AGENT_CORE_USERNAME` |
| `--core-password` | `ERU_AGENT_CORE_PASSWORD` |
| `--docker-endpoint` | `ERU_AGENT_DOCKER_ENDPOINT` |
| `--metrics-step` | `ERU_AGENT_METRICS_STEP` |
| `--metrics-transfers` | `ERU_AGENT_METRICS_TRANSFERS` |
| `--api-addr` | `ERU_AGENT_API_ADDR` |
| `--log-forwards` | `ERU_AGENT_LOG_FORWARDS` |
| `--log-stdout` | `ERU_AGENT_LOG_STDOUT` (`yes` to enable) |
| `--pidfile` | `ERU_AGENT_PIDFILE` |
| `--health-check-interval` | `ERU_AGENT_HEALTH_CHECK_INTERVAL` |
| `--health-check-timeout` | `ERU_AGENT_HEALTH_CHECK_TIMEOUT` |
| `--health-check-cache-ttl` | `ERU_AGENT_HEALTH_CHECK_CACHE_TTL` |
| `--heartbeat-interval` | `ERU_AGENT_HEARTBEAT_INTERVAL` |
| `--hostname` | `ERU_HOSTNAME` |
| `--check-only-mine` | — |

Two environment variables have no flag:

- `AGENT_IN_DOCKER` — set by the container image; makes the agent read host process state from `/hostProc`
- `ERU_AGENT_EXPERIMENTAL_FILTER` — set to `label` to filter workloads by label instead of by environment variable when `check_only_mine` is on

The agent prints its effective configuration to stdout at startup, so the fastest way to check what an override did is to read the first lines of its log.
