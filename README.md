# qSync
qSync fixes rsync's core flaw — one lost packet stalling an entire file batch — by giving each file its own independent QUIC stream over a single connection, so only the affected file slows down while everything else keeps flowing.

See [qSync_plan.md](qSync_plan.md) for the full phased execution plan.

## Status

- **Phase 0 (setup)** — Go module, `quic-go` transport, self-signed TLS cert
  helper ([internal/certutil](internal/certutil)), and a working QUIC
  connection between client and server binaries.
- **Phase 1 (manifest & control protocol)** — directory scanning +
  SHA-256 hashing ([internal/manifest](internal/manifest)), and a
  length-prefixed JSON control-stream protocol
  ([internal/protocol](internal/protocol)) for exchanging manifests.
- **Phase 2 (diffing)** — client/server manifest comparison
  ([internal/manifest/diff.go](internal/manifest/diff.go)) producing an
  add/update/delete plan, covered by unit tests with synthetic manifests.

Not yet implemented: concurrent per-file QUIC stream transfer (Phase 3),
reassembly/integrity checking (Phase 4), and everything from Phase 5 onward
(baseline comparison, benchmarking, analysis, polish).

## Building

```
go build ./...
```

## Running

Start the server, pointing it at the directory to serve:

```
go run ./cmd/server -addr 0.0.0.0:4433 -dir /path/to/serve
```

Run the client against a local directory to see the computed sync plan
(add / update / delete) — no files are transferred yet:

```
go run ./cmd/client -addr 127.0.0.1:4433 -dir /path/to/local/copy -delete
```

## Testing

```
go test ./...
```

