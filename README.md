# logseq-gw

A small HTTP gateway that lets Agent-of-Empires sandbox containers — which
have no local graph files — reach a `logseq` graph's headless
`db-worker-node` process, without a graph-sync step.

It runs inside the same container as the graph's `logseq` CLI installation
and root-dir, listens on one fixed port bound to `0.0.0.0`, and reverse
proxies path-routed requests to the loopback-bound worker(s) it starts and
tracks on your behalf.

Background and full design: see the `Sandbox graph gateway design` and
`Sandbox graph access options` documents in the `logseq-pm` project
(DirtyBits graph).

## Routes

- `POST /graphs/:graph/v1/invoke` — proxies to the graph's worker.
- `GET /graphs/:graph/v1/events` — SSE proxy, unbuffered, no idle timeout.
- `GET /graphs` — returns the configured allowlist as `{"graphs": [...]}`.
- `GET /healthz`

Any other path (notably the worker's unauthenticated `/v1/shutdown` and
`/v1/import-db-binary`) is never forwarded — it isn't registered.

## Configuration (environment variables)

| Var | Required | Default | Meaning |
|---|---|---|---|
| `GATEWAY_PORT` | no | `8085` | Listen port |
| `GATEWAY_ROOT_DIR` | yes | — | Passed to `logseq --root-dir`; must match where this container's graph files live |
| `GATEWAY_GRAPHS` | yes | — | Comma-separated graph allowlist |
| `GATEWAY_LOGSEQ_BIN` | no | `logseq` | logseq CLI executable |
| `GATEWAY_SUBPROCESS_TIMEOUT_MS` | no | `20000` | Timeout for each `logseq server start`/`server list` call |

## Client side

Point a client at the gateway with `LOGSEQ_CLI_BASE_URL=http://<gateway>/graphs/<graph>`.
The `logseq` CLI concatenates this with `/v1/invoke` / `/v1/events` itself, so
ordinary graph-scoped `logseq`/`logseq-pm` commands work unmodified. The
`logseq-pm`-side `--gw` flag that injects this per-call is a separate,
companion change — out of scope here.

## Run

```sh
go build -o logseq-gw .
GATEWAY_ROOT_DIR=~/logseq GATEWAY_GRAPHS=my-graph ./logseq-gw
```

## Test

```sh
go test ./...
```

Unit tests fake the `logseq` subprocess calls. Verifying against a real
`logseq` install and graph is manual: run the gateway, then run any
graph-scoped `logseq`/`logseq-pm` command with `LOGSEQ_CLI_BASE_URL` pointed
at `http://127.0.0.1:<port>/graphs/<graph>` and diff the output against a
direct (non-gateway) run.
