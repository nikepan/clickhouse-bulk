# Prometheus alerts

Endpoint: `GET /metrics` (optional prefix: `metrics_prefix`).

Backup metrics (`ch_bkp_*`) exist only when dual-write is enabled. Journal metrics exist only when `journal_dir` is set.

## Live ClickHouse

```yaml
- alert: ClickhouseBulkLiveNoGoodServers
  expr: ch_good_servers == 0
  for: 5m

- alert: ClickhouseBulkLiveQueueStuck
  expr: ch_send_queue > 100
  for: 10m

- alert: ClickhouseBulkLiveDumpsBacklog
  expr: ch_queued_dumps > 10
  for: 15m

- alert: ClickhouseBulkLiveDumpsCreated
  expr: increase(ch_dump_count[5m]) > 0
  for: 1m

- alert: ClickhouseBulkLiveDumpDirLarge
  expr: ch_dump_dir_bytes > 1e9
  for: 30m
```

## Backup ClickHouse (dual-write)

```yaml
- alert: ClickhouseBulkBackupNoGoodServers
  expr: ch_bkp_good_servers == 0
  for: 5m

- alert: ClickhouseBulkBackupQueueStuck
  expr: ch_bkp_send_queue > 100
  for: 10m

- alert: ClickhouseBulkBackupDumpsBacklog
  expr: ch_bkp_queued_dumps > 10
  for: 15m

- alert: ClickhouseBulkBackupDumpsCreated
  expr: increase(ch_bkp_dump_count[5m]) > 0
  for: 1m

- alert: ClickhouseBulkBackupDumpDirLarge
  expr: ch_bkp_dump_dir_bytes > 1e9
  for: 30m
```

## Journal (when `journal_dir` is set)

WAL pending = accepted by HTTP but not yet on live CH or live `dump_dir`.

```yaml
- alert: ClickhouseBulkJournalBacklog
  expr: ch_journal_pending > 1000
  for: 30m

- alert: ClickhouseBulkJournalDiskUsage
  expr: ch_journal_dir_bytes > 1e9
  for: 30m
```

If journal grows while `ch_send_queue` is high, consider lowering ingest or setting `clickhouse.send_max_rps`.

## Live vs backup drift (heuristic)

```yaml
- alert: ClickhouseBulkBackupSendLag
  expr: (increase(ch_sent_count[10m]) - increase(ch_bkp_sent_count[10m])) > 100
  for: 30m
```

Tune for your batch size. Authoritative checks: row counts or `system.parts` on both clusters.

## HTTP health

`GET /status` — JSON with `live` / `backup`: `queue_len`, per-server `bad`.

## Reconciliation

1. `dump_dir/failed/`, `bkp_dump_dir/failed/` — fix schema/params, then `POST /debug/replay-failed?target=live|backup|all`.
2. Logs: `SUCCESS: dump to file`, `dump moved to failed`, `journal replay`, `ERROR: Send`.
3. [DUAL_WRITE.md](./DUAL_WRITE.md) — semantics.
4. [RISKS.md](./RISKS.md) — risks by deployment mode.
