# qSync: Execution Plan (Phases 0–8)

One-line summary: qSync fixes rsync's core flaw — one lost packet stalling an entire
file batch — by giving each file its own independent QUIC stream over a single
connection, so only the affected file slows down while everything else keeps flowing.

---

## Phase 0 — Setup & tooling decisions
1. Language/QUIC library: **Go**, using **`quic-go`** (`github.com/quic-go/quic-go`)
   — the standard, actively maintained pure-Go QUIC implementation (RFC 9000/9001/9002),
   used in production by Caddy, cloudflared, AdGuardHome, etc.
2. Set up project repo as a Go module:
   - `qsync/cmd/client` (client binary, `main.go`)
   - `qsync/cmd/server` (server binary, `main.go`)
   - `qsync/internal/manifest` (scanning, hashing, diff)
   - `qsync/internal/protocol` (control-stream message framing/encoding)
   - `qsync/internal/transfer` (stream lifecycle, reassembly)
   - `qsync/bench` (benchmark driver + TCP baseline)
   - `go.mod` / `go.sum`
3. Generate a self-signed TLS cert/key (QUIC mandates TLS 1.3) — e.g. with
   `crypto/tls` + `crypto/x509` helper code, or `openssl`, loaded via
   `tls.Config` passed into `quic-go`.
4. Verify a basic QUIC "hello world" stream works between local client/server
   (`quic.Listen` / `quic.Dial`, open a stream, write/read bytes) before
   writing any sync logic.

## Phase 1 — Manifest & control protocol (sub-problem 1)
1. Define the manifest format per file (Go struct, e.g. `manifest.FileEntry`):
   relative path, size, mtime (optional), content hash (`crypto/sha256`, or
   `lukechampine.com/blake3` for speed).
2. Implement a directory scanner (`filepath.WalkDir`) producing this manifest
   as `[]manifest.FileEntry` — shared package used by both client and server.
3. Design the control stream (stream 0) protocol — encode messages with
   `encoding/gob` or a small length-prefixed JSON/`encoding/binary` framing:
   - `MANIFEST_REQUEST` / `MANIFEST_RESPONSE`
   - `SYNC_PLAN` (files to transfer + assigned stream IDs)
   - `FILE_COMPLETE` acks (optional)
4. Server: on `quic.Connection.AcceptStream`, opens stream 0, sends its manifest.
5. Client: `OpenStreamSync` for stream 0, receives server manifest, scans
   local directory, computes diff.

## Phase 2 — Diffing logic (sub-problem 2)
1. Compare client vs server manifest by path:
   - Missing on client → added
   - Hash mismatch → modified
   - Present on client, absent on server → optional delete flag
2. Output: an ordered/unordered transfer list.
3. Unit test the diff logic in isolation with synthetic manifests (no network).

## Phase 3 — Concurrent file transfer over QUIC streams (sub-problem 3)
1. For each file in the transfer list, client calls `conn.OpenStreamSync(ctx)`
   and sends `FILE_REQUEST(path)`.
2. Server responds on that stream: size header + raw bytes (`io.Copy` from
   the file into the `quic.Stream`).
3. Launch all file requests concurrently using **goroutines** + a
   `sync.WaitGroup` (or `errgroup.Group` from `golang.org/x/sync/errgroup`) —
   don't accidentally serialize streams by blocking on one before opening
   the next.
4. Respect a configurable max-concurrent-streams cap using a **buffered
   channel as a semaphore** (or `golang.org/x/sync/semaphore`); also check
   `quic-go`'s `MaxIncomingStreams` transport parameter on the server side.

## Phase 4 — Reassembly & integrity (sub-problem 4)
1. Client writes each incoming stream to a temp file (`os.CreateTemp` in the
   destination directory) using `io.Copy` combined with `io.TeeReader` into
   a hasher.
2. On stream completion (`io.EOF`), compare the running hash to the manifest
   hash.
3. On mismatch: retry via a new stream (`OpenStreamSync` again), or flag as
   failed.
4. On success: atomically rename temp file into place with `os.Rename`.
5. Track per-file start/end timestamps (`time.Now()`) for later benchmark
   plots — store in a struct and write out as CSV/JSON with
   `encoding/csv` / `encoding/json`.

## Phase 5 — Baseline for comparison
1. Implement/reuse a TCP-based baseline:
   - shell out to real `rsync` over SSH/TCP via `os/exec`, or
   - write a minimal single-`net.Conn` (plain TCP) equivalent in Go sending
     files sequentially over one byte stream — reuse the same manifest/diff
     packages from Phase 1–2 so the comparison is apples-to-apples.
2. Baseline emits per-file completion timestamps in the same log format as
   qSync (same CSV/JSON schema) for direct comparability.

## Phase 6 — Network impairment & benchmark harness
1. Use `tc netem` to inject packet loss (loopback, netns, or containers for
   a more realistic virtual link).
2. Repeatable test scenario:
   - N files (100–1000), mixed sizes.
   - Loss rates: 0%, 1%, 5%, 10%.
   - Run qSync and baseline at each loss rate, capture per-file timestamps.
3. Automate with a driver — either a shell script (`bench/run_experiment.sh`)
   invoking the compiled Go binaries with different flags, or a small Go
   program in `qsync/bench` that shells out via `os/exec` to run each
   trial and collect results.

## Phase 7 — Analysis & visualization
1. Aggregate the CSV/JSON logs into `(tool, loss_rate, file_id, start_time, end_time)`.
2. Plotting: since Go's plotting ecosystem is thinner than Python's, either
   - use **`gonum.org/v1/plot`** to generate PNG/SVG charts directly in Go, or
   - export the aggregated CSV and plot with a small separate Python/matplotlib
     script (kept out of the Go module, purely for the report).
3. Key plots:
   - Total sync time vs. loss rate (qSync vs baseline).
   - Per-file completion timeline at a fixed loss rate (e.g. 5%) — the plot
     that visually proves the HoL-blocking claim.
4. Write up the quantitative result (e.g. "at 5% loss, qSync completed 95% of
   files within X seconds vs. Y seconds for baseline").

## Phase 8 — Polish & write-up
1. README with architecture diagram (control stream + N file streams).
2. Precise documentation of the manifest/control protocol wire format.
3. Short demo recording showing qSync completing while baseline stalls under
   injected loss.
4. Stretch goals (optional): resumable transfers, compression
   (`compress/gzip` or `zstd` via `klauspost/compress`), rsync-style
   block/delta diffing, deletion propagation, directory watch
   (`fsnotify/fsnotify`) for incremental mode.

---

## Go-specific dependency summary
- `github.com/quic-go/quic-go` — QUIC transport (core dependency)
- `golang.org/x/sync/errgroup` and/or `golang.org/x/sync/semaphore` — concurrency control
- `lukechampine.com/blake3` (optional) — fast hashing, or stick with stdlib `crypto/sha256`
- `github.com/klauspost/compress` (optional, stretch goal) — compression
- `github.com/fsnotify/fsnotify` (optional, stretch goal) — directory watching
- `gonum.org/v1/plot` (optional) — native Go plotting for Phase 7
- Everything else (manifest scanning, framing, CSV/JSON logging, TLS cert
  generation) can be done with the Go standard library alone.
  
