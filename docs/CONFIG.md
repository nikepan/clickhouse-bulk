# Configuration reference

Config file: JSON (default `config.json`). Override via environment variables (Docker-friendly).

**Precedence:** defaults → JSON file → environment variables.

**Local data paths** (`journal_dir`, `dump_dir`, `bkp_dump_dir`): validated at startup — paths are `filepath.Clean`’d; values containing `..` are rejected. Empty `journal_dir` disables the journal.

**Dump file ids** (replay / read): only a basename or `failed/<basename>` is allowed; `..` and nested paths are rejected (`ErrInvalidDumpID`).

## Top-level

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `listen` | — | `:8124` | HTTP listen address |
| `flush_count` | `CLICKHOUSE_FLUSH_COUNT` | `10000` | Rows per batch before flush |
| `flush_interval` | `CLICKHOUSE_FLUSH_INTERVAL` | `1000` | Flush interval (ms) |
| `clean_interval` | `CLICKHOUSE_CLEAN_INTERVAL` | `0` | Remove idle internal tables (ms); `0` = off |
| `remove_query_id` | `CLICKHOUSE_REMOVE_QUERY_ID` | `true` | Strip `query_id` for batching |
| `opaque_insert` | `OPAQUE_INSERT` | `false` | If `true`, every INSERT bypasses batching (opaque passthrough). Default: auto for `FORMAT Native` / `RowBinary` / … and `Content-Type: application/octet-stream` |
| `dump_check_interval` | `DUMP_CHECK_INTERVAL` | `300` | Live dump replay period (s); `-1` = off |
| `bkp_dump_check_interval` | `BKP_DUMP_CHECK_INTERVAL` | `0` | Backup dump replay (s); `0` = use `dump_check_interval` |
| `dump_replay_batch` | `DUMP_REPLAY_BATCH` | `0` | Max dump files replayed per tick per target; `0` = unlimited |
| `max_dump_files` | `MAX_DUMP_FILES` | `0` | Max pending `.dmp` per dir; oldest pruned; `0` = unlimited |
| `dump_dir` | `DUMP_DIR` | `dumps` | Live failed-batch directory |
| `bkp_dump_dir` | `CLICKHOUSE_BKP_DUMP_DIR` | `dumps-bkp` | Backup failed-batch directory |
| `journal_dir` | `JOURNAL_DIR` | `""` | WAL directory; empty = journal disabled |
| `journal_fsync` | `JOURNAL_FSYNC` | `false` | `fsync` after each WAL append |
| `max_journal_pending` | `MAX_JOURNAL_PENDING` | `0` | Max unacked WAL rows; `0` = unlimited; full → HTTP 503 |
| `shutdown_drain_sec` | `SHUTDOWN_DRAIN_SEC` | `60` | Max time to flush queues on SIGTERM/SIGINT |
| `debug` | `CLICKHOUSE_BULK_DEBUG` | `false` | Log request metadata (redacted params, byte counts); not full row data |
| `log_queries` | `LOG_QUERIES` | `false` | Log each insert batch sent to CH |
| `metrics_prefix` | `METRICS_PREFIX` | `""` | Prometheus metric name prefix |
| `use_tls` | — | `false` | TLS for HTTP listener |
| `tls_cert_file` / `tls_key_file` | — | `""` | Listener certificate |

## `clickhouse` (live)

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `servers` | `CLICKHOUSE_SERVERS` | `http://127.0.0.1:8123` | Comma-separated URLs (trimmed) |
| `down_timeout` | `CLICKHOUSE_DOWN_TIMEOUT` | `60` | Retry server marked bad after (s) |
| `connect_timeout` | `CLICKHOUSE_CONNECT_TIMEOUT` | `10` | HTTP client timeout (s) |
| `tls_server_name` | `CLICKHOUSE_TLS_SERVER_NAME` | `""` | TLS ServerName |
| `insecure_tls_skip_verify` | `CLICKHOUSE_INSECURE_TLS_SKIP_VERIFY` | `false` | Skip TLS verify |
| `send_max_rps` | `CLICKHOUSE_SEND_MAX_RPS` | `0` | Max POSTs/s to live; `0` = unlimited |
| `send_max_burst` | `CLICKHOUSE_SEND_MAX_BURST` | `0` | Burst tokens; default ≈ `send_max_rps` if `0` |

## `clickhouse-backup` (optional)

Enabled when `servers` is non-empty after config merge, or `CLICKHOUSE_BACKUP_SERVERS` is set.

Same fields as live, plus:

| Key | Env | Description |
|-----|-----|-------------|
| `query_params` | `CLICKHOUSE_BACKUP_QUERY_PARAMS` | Appended to every backup request (`database=standby`, …) |
| `send_max_rps` | `CLICKHOUSE_BACKUP_SEND_MAX_RPS` | Backup send rate limit |
| `send_max_burst` | `CLICKHOUSE_BACKUP_SEND_MAX_BURST` | Backup burst |

Env for backup timeouts/TLS: `CLICKHOUSE_BACKUP_DOWN_TIMEOUT`, `CLICKHOUSE_BACKUP_CONNECT_TIMEOUT`, `CLICKHOUSE_BACKUP_TLS_SERVER_NAME`, `CLICKHOUSE_BACKUP_INSECURE_TLS_SKIP_VERIFY`.

## Sample files

| File | Use case |
|------|----------|
| `config.sample.json` | Live only + journal |
| `config.sample-backup.json` | Live + backup dual-write |

## Related docs

- [DUAL_WRITE.md](./DUAL_WRITE.md) — semantics, journal, dumps
- [ALERTS.md](./ALERTS.md) — Prometheus alert examples
