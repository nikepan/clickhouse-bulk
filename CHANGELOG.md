# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Docs: [docs/CLIENT_COMPATIBILITY.md](docs/CLIENT_COMPATIBILITY.md) (clickhouse-go, clickhouse-connect); P4 roadmap in [Plan.md](Plan.md).
- Dependabot: `.github/dependabot.yml` (gomod, github-actions, docker; weekly).
- **Live / backup dual-write** when `clickhouse-backup.servers` or `CLICKHOUSE_BACKUP_SERVERS` is set.
- `DualSender`, separate queues, dumps (`dump_dir`, `bkp_dump_dir`), and replay loops.
- **Journal (WAL):** `journal_dir`, `journal_fsync`, `max_journal_pending`; ack on live send or live dump; replay on startup; metrics `ch_journal_pending`, `ch_journal_dir_bytes`.
- **Send rate limit:** `send_max_rps`, `send_max_burst` per `clickhouse` and `clickhouse-backup`.
- `clickhouse-backup.query_params`, `bkp_dump_check_interval`, `dump_replay_batch`, `max_dump_files`, `shutdown_drain_sec`.
- `GET /status` with live/backup health.
- `POST|GET /debug/replay-failed` — replay dumps from `dump_dir/failed/` (and `bkp_dump_dir/failed/`).
- Prometheus: `ch_bkp_*`, `ch_send_queue`, `ch_dump_dir_bytes`, `ch_last_sent_unixtime`, etc.
- Docs: [docs/DUAL_WRITE.md](docs/DUAL_WRITE.md), [docs/RISKS.md](docs/RISKS.md), [docs/CONFIG.md](docs/CONFIG.md), [docs/ALERTS.md](docs/ALERTS.md).
- Samples: `config.sample.json` (live), `config.sample-backup.json` (dual-write).

### Fixed

- Graceful shutdown: HTTP stop → `SafeQuit` → drain live/backup queues (`shutdown_drain_sec`).
- 4xx dumps moved to `failed/` (no infinite retry).
- `CleanTables` / mutex fixes; server URL validation and trim.
- Backup metrics registered only when dual-write is enabled.

### Changed

- Go **1.26.3**; dependencies updated (echo v4.15.2, prometheus client_golang v1.23.2, testify v1.11.1, golang.org/x/*).
- Go module: `github.com/itcrow/clickhouse-bulk` (was `github.com/nikepan/clickhouse-bulk`).
- `RunServer` builds per-target senders; backup wrapped in `DualSender`.
- Journal ack when live stores batch (CH **or** `dump_dir`), not backup-only.

### Notes

- Dual-write is **best-effort per target**, not synchronous replication. See [Plan.md](Plan.md).
- Fork base: upstream **v1.3.9** ([nikepan/clickhouse-bulk](https://github.com/nikepan/clickhouse-bulk)).

---

## Upstream (nikepan/clickhouse-bulk)

See [upstream releases](https://github.com/nikepan/clickhouse-bulk/releases) for history before this fork.
