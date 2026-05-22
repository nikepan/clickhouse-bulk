# Roadmap and known issues

Code review of clickhouse-bulk (live + `clickhouse-backup` mode).  
Dual-write semantics: **asynchronous replication with at-least-once delivery per target**, not a guarantee that live ≡ backup.

See [DUAL_WRITE.md](./DUAL_WRITE.md), [RISKS.md](./RISKS.md), [CONFIG.md](./CONFIG.md), [ALERTS.md](./ALERTS.md), and [CLIENT_COMPATIBILITY.md](./CLIENT_COMPATIBILITY.md).

---

## P0 — critical (operations / data loss)

### 1. HTTP `200 OK` before data is actually written — **done**

- **Status:** ✅ **Journal (WAL)** when `journal_dir` is set: append before `200`, replay unacked on startup, `ack` when live batch is on ClickHouse **or** in `dump_dir` (not backup-only). Empty `journal_dir` = legacy behavior.

### 2. Graceful shutdown does not drain queues — **done**

- **Status:** ✅ HTTP shutdown → `SafeQuit` → exit; drain timeout via `shutdown_drain_sec`.

### 3. No coordination between live and backup — **done (docs + metrics)**

- **Status:** ✅ [DUAL_WRITE.md](./DUAL_WRITE.md), [ALERTS.md](./ALERTS.md); metrics `ch_last_sent_unixtime`, `ch_bkp_last_sent_unixtime` for lag heuristics.

### 4. 4xx errors in dumps — infinite retry — **done**

- **Status:** ✅ Dumps with filename kind `2` (4xx) moved to `<dump_dir>/failed/`, not retried.

---

## P1 — high (backup / config / observability)

### 5. Double memory and load in backup mode — **documented**

- **Status:** ✅ Documented in [DUAL_WRITE.md](./DUAL_WRITE.md); queue size 1000 per target.

### 6. Dump replay bypasses the send queue — **open**

- **Status:** ⬜ Replay stays synchronous via `SendQuery` (correct delete-after-success). Rate limit optional later.

### 7. Single `params` set for live and backup — **done**

- **Status:** ✅ `clickhouse-backup.query_params` / `CLICKHOUSE_BACKUP_QUERY_PARAMS`.

### 8. `config.sample.json` always enables backup — **done**

- **Status:** ✅ Backup example moved to `config.sample-backup.json`.

### 9. Env: spaces in server lists — **done**

- **Status:** ✅ `splitTrimServers` / `normalizeServerList`.

### 10. Env vs file order for backup TLS — **done**

- **Status:** ✅ Documented in [DUAL_WRITE.md](./DUAL_WRITE.md#configuration-precedence).

---

## P2 — medium (code quality / UX)

### 11. `defer delete` inside a loop (`CleanTables`) — **done**

- **Status:** ✅ Collect keys, delete after loop (`collector.go`, `tablesCleanHandler`).

### 12. `CleanTable`: `t = nil` does not remove from the map — **done**

- **Status:** ✅ Caller deletes from map; `CleanTable` only stops ticker.

### 13. `Table.Flush` while holding the mutex — **done**

- **Status:** ✅ `doFlush` releases lock before `Sender.Send`.

### 14. `/status` does not expose backup — **done**

- **Status:** ✅ `GET /status` returns `FullStatus` with `live` and `backup` targets.

### 15. `WaitFlush` / `wg` on queue failure — **done**

- **Status:** ✅ `SafeQuit` honors `shutdown_drain_sec` timeout.

---

## P3 — low (observability / convenience)

### 16. Empty URL in `servers` — **done**

- **Status:** ✅ `validateClickhouseConfig` at startup.

### 17. No limit on dump directory size — **done**

- **Status:** ✅ `max_dump_files` prunes oldest pending `.dmp` files; metrics `ch_dump_dir_bytes`, `ch_bkp_dump_dir_bytes`.

### 18. `LockedFiles` on delete failure — **done**

- **Status:** ✅ `DeleteDump` retries remove 3 times.

### 19. No health check for “backup lagging behind live” — **done**

- **Status:** ✅ `ch_last_sent_unixtime` vs `ch_bkp_last_sent_unixtime`; alert example in [ALERTS.md](./ALERTS.md).

### 20. `ch_bkp_*` metrics when backup mode is off — **done**

- **Status:** ✅ `InitMetrics(prefix, backupEnabled)` registers backup collectors only when dual-write is on.

### 21. Parallel I/O from two dump `Listen` loops — **done**

- **Status:** ✅ `bkp_dump_check_interval` (fallback: `dump_check_interval`); `dump_replay_batch` caps files per replay tick.

---

## Dual-write scenarios (reference)

| Situation | Outcome |
|-----------|---------|
| Live OK, backup down | Data on live; backup catches up from `bkp_dump_dir` |
| Live down → dump, backup OK | Data on backup; live catches up from `dump_dir` |
| Live OK, backup OK, different latency | Temporary replica divergence |
| Different schema/permissions on backup | 4xx → `bkp_dump_dir/failed/` |

---

## Recommended work order

1. ~~P0.02 shutdown~~ ✅  
2. ~~P0.03 docs/alerts~~ ✅  
3. ~~P0.04 4xx DLQ~~ ✅  
4. ~~P1.07–10 config~~ ✅  
5. ~~P2.11–15 code/UX~~ ✅  
6. P1.06 replay rate limit (optional)  
7. ~~P0.01 journal~~ ✅  
8. ~~P3.17, P3.20–21~~ ✅  

---

## Live/backup implementation status

| Component | Status |
|-----------|--------|
| `DualSender` | ✅ |
| Separate dumps + replay | ✅ |
| Metrics + last-sent timestamps | ✅ |
| `/status` live + backup | ✅ |
| `query_params` for backup | ✅ |
| `config.sample-backup.json` | ✅ |
| Journal (P0.01) | ✅ |
| Roadmap items (open above) | P1.06, P4 (client compatibility) |

---

## P4 — Client compatibility (optional)

Goal: improve interoperability with [clickhouse-go](https://github.com/ClickHouse/clickhouse-go) and [clickhouse-connect](https://clickhouse.com/docs/integrations/python) **without** turning bulk into a full HTTP proxy. Current behaviour: [CLIENT_COMPATIBILITY.md](./CLIENT_COMPATIBILITY.md).

Design principle: **default path unchanged** (batched text INSERT for Vector/curl); new behaviour behind config flags.

### P4.1 — Opaque INSERT passthrough — open

- **Problem:** Drivers send `INSERT … FORMAT Native` + `application/octet-stream` (or full query+body blob). Bulk’s `ParseQuery` / `Collector` only merge **text** `FORMAT` / `VALUES` rows.
- **Proposal:**
  - Detect passthrough: e.g. `Content-Type: application/octet-stream`, or `FORMAT Native` / `RowBinary` in query, or config `opaque_insert: true` for all INSERTs.
  - Skip collector batching: after optional journal `Append`, enqueue one `ClickhouseRequest` with **raw** URL params + body (or forward client body verbatim).
  - Still async `200` + empty body (unless P4.5).
- **Effort:** ~2–4 days.
- **Unlocks:** clickhouse-go HTTP `PrepareBatch`; connect `insert()` body format (still no sync errors).

### P4.2 — Request decompression — open

- **Problem:** clickhouse-go (LZ4/ZSTD) and clickhouse-connect (`compress=True`) send `Content-Encoding` / CH `decompress=1` settings. Bulk reads body as plain text.
- **Proposal:** If `Content-Encoding` or `decompress` setting present, decompress in `writeHandler` before routing; config `max_request_bytes`.
- **Effort:** ~2–3 days (add deps: klauspost/compress or std for gzip).
- **Depends on:** P4.1 for Native payloads.

### P4.3 — Response header forwarding — open

- **Problem:** connect reads `X-ClickHouse-Summary`, `X-ClickHouse-Query-Id`; bulk returns only status + body on proxied queries; INSERT returns empty body.
- **Proposal:**
  - On **proxied** (`SendQuery`, non-insert): copy CH response headers to Echo response.
  - On **passthrough INSERT** (P4.1): optional forward headers if we switch to sync wait (P4.5) or fire-and-forget with empty body (limited value).
- **Effort:** ~1 day (proxied only); +1–2 days with passthrough sync.
- **Code touch:** `ClickhouseServer.SendQuery` return headers; `writeHandler` set `c.Response().Header()`.

### P4.4 — Hybrid batch formats (config) — open

- **Problem:** Want TabSeparated batched for ETL, Native passthrough for apps.
- **Proposal:** Config e.g. `batch_formats: ["TabSeparated","Values","JSONEachRow"]`; other formats → P4.1 path.
- **Effort:** ~2–3 days after P4.1.
- **Tests:** Matrix format × Content-Type.

### P4.5 — Optional synchronous INSERT — open

- **Problem:** Drivers expect CH HTTP semantics: error in response, not silent queue success.
- **Proposal:** `sync_insert: true` or request header `X-Bulk-Sync: 1`: do not batch; `SendQuery` inline; return CH status/body/headers; journal ack after CH success (or dump). Dual-write: define policy (sync live only, backup async).
- **Effort:** ~1–2 weeks (journal, timeouts, metrics, dual-write semantics).
- **Risk:** Defeats throughput; document as debug / low-rate only.

### P4.6 — Documentation & samples — partial

- **Status:** ✅ [CLIENT_COMPATIBILITY.md](./CLIENT_COMPATIBILITY.md).
- **Todo:** Optional `examples/go_direct_ch.go`, `examples/python_raw_insert.py` (non-blocking).

### Recommended implementation order

1. P4.6 (docs) ✅  
2. P4.1 opaque passthrough  
3. P4.2 decompression  
4. P4.4 hybrid formats  
5. P4.3 headers (proxied, then passthrough if sync)  
6. P4.5 sync insert (only if product needs driver-drop-in)

### Non-goals

- Native TCP on bulk port.
- Merging multiple Native INSERT bodies into one batch.
- Exactly-once or full `clickhouse-connect` feature parity (sessions, temporary tables, external data) without explicit design.
