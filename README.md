# ClickHouse-Bulk (live / standby fork)

[![build](https://github.com/itcrow/clickhouse-bulk/actions/workflows/test.yml/badge.svg)](https://github.com/itcrow/clickhouse-bulk/actions/workflows/test.yml)
[![download binaries](https://img.shields.io/badge/binaries-download-blue.svg)](https://github.com/itcrow/clickhouse-bulk/releases)

HTTP insert collector for [ClickHouse](https://clickhouse.com/). Batches INSERTs and sends them to one or two ClickHouse endpoints (live + optional standby).

Based on [nikepan/clickhouse-bulk](https://github.com/nikepan/clickhouse-bulk) with **dual-write**, **journal (WAL)**, separate dumps, and extended metrics.

## Features

- Batch by query key (`flush_count`, `flush_interval`)
- Multiple servers per target with failover
- **Live + backup** async dual-write (`clickhouse-backup`)
- **Journal** — durable HTTP `200` before ClickHouse (`journal_dir`)
- On failure — spool to `dump_dir` / `bkp_dump_dir` with automatic replay
- 4xx batches → `failed/` (no infinite retry)
- **Send rate limit** per target (`send_max_rps`)
- Prometheus metrics + `GET /status`
- Graceful shutdown with queue drain

## Quick start

```bash
cp config.sample.json config.json   # live + journal
# or: cp config.sample-backup.json config.json
go build
./clickhouse-bulk -config config.json
```

Send INSERTs to `http://127.0.0.1:8124` (not the native ClickHouse port unless you proxy).

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/DUAL_WRITE.md](docs/DUAL_WRITE.md) | Architecture, journal, dumps, guarantees |
| [docs/RISKS.md](docs/RISKS.md) | Operational risks by mode (live / journal / dual-write) |
| [docs/CONFIG.md](docs/CONFIG.md) | Full config and env reference |
| [docs/ALERTS.md](docs/ALERTS.md) | Prometheus alert examples |
| [CHANGELOG.md](CHANGELOG.md) | Change history |
| [Plan.md](Plan.md) | Known issues / roadmap |

## Modes

### Live only (default sample)

`config.sample.json` — one ClickHouse, optional journal, no backup.

### Live + backup

`config.sample-backup.json` or:

```json
"clickhouse-backup": {
  "servers": ["http://standby:8123"],
  "query_params": "database=standby"
}
```

Each batch: live queue first, then backup queue. Separate dumps and replay intervals.

### Journal off

Set `"journal_dir": ""` or unset — legacy behavior (HTTP `200` before in-memory accept only).

## Configuration (summary)

```json
{
  "listen": ":8124",
  "flush_count": 10000,
  "flush_interval": 1000,
  "dump_dir": "dumps",
  "dump_check_interval": 300,
  "dump_replay_batch": 10,
  "journal_dir": "journal",
  "max_journal_pending": 1000000,
  "shutdown_drain_sec": 60,
  "clickhouse": {
    "servers": ["http://127.0.0.1:8123"],
    "send_max_rps": 0,
    "send_max_burst": 0
  }
}
```

See [docs/CONFIG.md](docs/CONFIG.md) for all keys and environment variables.

## Environment variables (Docker)

Common overrides:

| Variable | Purpose |
|----------|---------|
| `CLICKHOUSE_SERVERS` | Live URLs (comma-separated) |
| `CLICKHOUSE_BACKUP_SERVERS` | Enable backup + URLs |
| `JOURNAL_DIR` | WAL path (`""` disables) |
| `MAX_JOURNAL_PENDING` | WAL backlog cap (503 when full) |
| `CLICKHOUSE_SEND_MAX_RPS` | Live send rate limit |
| `CLICKHOUSE_BACKUP_SEND_MAX_RPS` | Backup send rate limit |
| `DUMP_DIR` / `CLICKHOUSE_BKP_DUMP_DIR` | Dump directories |
| `SHUTDOWN_DRAIN_SEC` | Shutdown drain timeout |

Full list: [docs/CONFIG.md](docs/CONFIG.md).

## Metrics

```bash
curl -s http://127.0.0.1:8124/metrics | grep '^ch_'
```

| Metric | Description |
|--------|-------------|
| `ch_received_count` | HTTP inserts received |
| `ch_sent_count` / `ch_bkp_sent_count` | Batches sent per target |
| `ch_send_queue` / `ch_bkp_send_queue` | Sender queue depth |
| `ch_queued_dumps` / `ch_bkp_queued_dumps` | Pending dump files |
| `ch_dump_dir_bytes` / `ch_bkp_dump_dir_bytes` | Dump dir size on disk |
| `ch_journal_pending` | Unacked WAL rows (if journal on) |
| `ch_journal_dir_bytes` | Journal dir size |
| `ch_last_sent_unixtime` / `ch_bkp_last_sent_unixtime` | Last successful send |

`GET /status` — live/backup health JSON.

## Example batching

```sql
INSERT INTO t (a,b) VALUES ('1','2')
INSERT INTO t (a,b) VALUES ('3','4')
```

→ one POST to ClickHouse:

```sql
INSERT INTO t (a,b) VALUES ('1','2')('3','4')
```

## Tips

- Use uppercase `FORMAT` and `VALUES` for best performance.
- Set `remove_query_id: true` if the driver sends `query_id` (breaks batching).
- Tune `send_max_rps` when ClickHouse or journal backlog cannot keep up.

## Release CI (tags `*.*.*`)

### GitHub Release 403

If GoReleaser fails with `403 Resource not accessible by integration` on release upload:

1. **Settings → Actions → General → Workflow permissions** → **Read and write permissions** (not read-only).
2. Re-run the workflow after pushing an updated `release.yml` (needs `permissions: contents: write`).
3. Or create a [classic PAT](https://github.com/settings/tokens) with scope **`repo`**, add secret **`GH_PAT`**, re-run.

Delete a broken draft release on GitHub (**Releases**) before re-tagging if a previous run left a partial release.

### Docker Hub

Repository **Secrets** (Settings → Secrets and variables → Actions):

| Name | Required | Example | Purpose |
|------|----------|---------|---------|
| `DOCKERHUB_TOKEN` | **yes** | *(token)* | [Docker Hub access token](https://hub.docker.com/settings/security) |
| `DOCKERHUB_USERNAME` | no | `itcrow` | Hub login; if unset, CI uses GitHub org name (`repository_owner`) |

Docker images are published as `itcrow/clickhouse-bulk` — Hub user must match (org `itcrow` or override via `DOCKERHUB_USERNAME`).

### CI build (optional Codacy)

Tests always run on push/PR. Codacy upload runs only if secret **`CODACY_PROJECT_TOKEN`** is set.

If upload fails with `Request URL not found`:

1. Add the repository at [Codacy](https://app.codacy.com) (org `itcrow`, repo `clickhouse-bulk`).
2. **Settings → Integrations → Repository API** → generate a **Project API token** for this repo.
3. Put it in GitHub secret `CODACY_PROJECT_TOKEN`, or **delete** the secret to skip Codacy entirely.

## Installation

- [Releases](https://github.com/itcrow/clickhouse-bulk/releases)
- [Docker](https://hub.docker.com/r/itcrow/clickhouse-bulk/)
- From source (Go 1.26+): `go build`

## License

See [LICENSE](LICENSE). Upstream: v1.3.9.
