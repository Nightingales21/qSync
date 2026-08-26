# qSync: Execution Plan (Phases 0–8)

One-line summary: qSync fixes rsync's core flaw — one lost packet stalling an entire
file batch — by giving each file its own independent QUIC stream over a single
connection, so only the affected file slows down while everything else keeps flowing.

---

## Phase 0 — Setup & tooling decisions
1. Pick a language/QUIC library:
   - **Rust + `quinn`** — mature, better raw performance for benchmark numbers.
   - **Python + `aioquic`** — faster to prototype and debug.
2. Set up project repo structure:
   - `qsync/client`
   - `qsync/server`
   - `qsync/common` (manifest, protocol)
   - `qsync/bench`
3. Generate a self-signed TLS cert/key (QUIC mandates TLS 1.3).
4. Verify a basic QUIC "hello world" stream works between local client/server
   before writing any sync logic.

## Phase 1 — Manifest & control protocol (sub-problem 1)
1. Define the manifest format per file: relative path, size, mtime (optional),
   content hash (SHA-256 or BLAKE3).
2. Implement a directory scanner producing this manifest (shared code, client
   and server side).
3. Design the control stream (stream 0) protocol — a framed message format for:
   - `MANIFEST_REQUEST` / `MANIFEST_RESPONSE`
   - `SYNC_PLAN` (files to transfer + assigned stream IDs)
   - `FILE_COMPLETE` acks (optional)
4. Server: on connection, opens stream 0, sends its manifest.
5. Client: receives server manifest, scans local directory, computes diff.

## Phase 2 — Diffing logic (sub-problem 2)
1. Compare client vs server manifest by path:
   - Missing on client → added
   - Hash mismatch → modified
   - Present on client, absent on server → optional delete flag
2. Output: an ordered/unordered transfer list.
3. Unit test the diff logic in isolation with synthetic manifests (no network).

## Phase 3 — Concurrent file transfer over QUIC streams (sub-problem 3)
1. For each file in the transfer list, client opens a new QUIC stream and
   sends `FILE_REQUEST(path)`.
2. Server responds on that stream: size header + raw bytes.
3. Launch all file requests concurrently (asyncio tasks / tokio tasks) —
   don't accidentally serialize streams by awaiting one before opening the next.
4. Respect a configurable max-concurrent-streams cap.

## Phase 4 — Reassembly & integrity (sub-problem 4)
1. Client writes each incoming stream to a temp file at the destination path.
2. On stream completion, hash received bytes and compare to manifest hash.
3. On mismatch: retry via a new stream, or flag as failed.
4. On success: atomically rename temp file into place.
5. Track per-file start/end timestamps for later benchmark plots.

## Phase 5 — Baseline for comparison
1. Implement/reuse a TCP-based baseline:
   - wrap real `rsync` over SSH/TCP, or
   - write a minimal single-TCP-connection equivalent sending files
     sequentially over one byte stream.
2. Baseline emits per-file completion timestamps in the same log format as
   qSync for direct comparability.

## Phase 6 — Network impairment & benchmark harness
1. Use `tc netem` to inject packet loss (loopback, netns, or containers for
   a more realistic virtual link).
2. Repeatable test scenario:
   - N files (100–1000), mixed sizes.
   - Loss rates: 0%, 1%, 5%, 10%.
   - Run qSync and baseline at each loss rate, capture per-file timestamps.
3. Automate with a driver script (`bench/run_experiment.sh` or Python) for
   reproducibility.

## Phase 7 — Analysis & visualization
1. Aggregate logs into `(tool, loss_rate, file_id, start_time, end_time)`.
2. Key plots:
   - Total sync time vs. loss rate (qSync vs baseline).
   - Per-file completion timeline at a fixed loss rate (e.g. 5%) — the plot
     that visually proves the HoL-blocking claim.
3. Write up the quantitative result (e.g. "at 5% loss, qSync completed 95% of
   files within X seconds vs. Y seconds for baseline").

## Phase 8 — Polish & write-up
1. README with architecture diagram (control stream + N file streams).
2. Precise documentation of the manifest/control protocol wire format.
3. Short demo recording showing qSync completing while baseline stalls under
   injected loss.
4. Stretch goals (optional): resumable transfers, compression, rsync-style
   block/delta diffing, deletion propagation, directory watch/incremental mode.
