# Docker

## Registry

Published images on [Docker Hub — `itcrow/clickhouse-bulk`](https://hub.docker.com/r/itcrow/clickhouse-bulk/).

| Tag | When |
|-----|------|
| `latest` | Latest release (multi-arch manifest) |
| `1.2.3` | Release tag (semver) |
| `1.2.3-amd64` / `1.2.3-arm64` | Per-architecture images (before manifest merge) |

Releases are built by [GoReleaser](../.github/workflows/release.yml) on git tags matching `*.*.*`.

---

## Quick run

Bulk listens on **8124** by default (`listen` in config). The container ships with `config.sample.json` (live + journal); override for production.

```bash
docker pull itcrow/clickhouse-bulk:latest

docker run -d --name clickhouse-bulk \
  -p 8124:8124 \
  -e CLICKHOUSE_SERVERS=http://host.docker.internal:8123 \
  -v "$(pwd)/config.json:/app/config.json:ro" \
  -v clickhouse-bulk-dumps:/app/dumps \
  -v clickhouse-bulk-journal:/app/journal \
  itcrow/clickhouse-bulk:latest \
  ./clickhouse-bulk -config=/app/config.json
```

Send INSERTs to `http://localhost:8124/` (not ClickHouse port 8123 unless you proxy).

Health / metrics:

```bash
curl -s http://127.0.0.1:8124/status
curl -s http://127.0.0.1:8124/metrics | head
```

---

## Configuration in Docker

### Mount a config file (recommended)

```bash
cp config.sample.json config.json
# edit clickhouse.servers, journal_dir, etc.

docker run -d --name clickhouse-bulk \
  -p 8124:8124 \
  -v "$(pwd)/config.json:/app/config.json:ro" \
  -v clickhouse-bulk-data:/app/dumps \
  -v clickhouse-bulk-journal:/app/journal \
  itcrow/clickhouse-bulk:latest \
  ./clickhouse-bulk -config=/app/config.json
```

### Environment variables only

Many settings can be set without editing JSON — see [CONFIG.md](./CONFIG.md). Example (live only, no journal file on disk):

```bash
docker run -d --name clickhouse-bulk \
  -p 8124:8124 \
  -e CLICKHOUSE_SERVERS=http://clickhouse:8123 \
  -e JOURNAL_DIR= \
  -v clickhouse-bulk-dumps:/app/dumps \
  itcrow/clickhouse-bulk:latest
```

Note: the default `ENTRYPOINT` uses `config.sample.json` inside the image; env vars **override** values from that file when set.

### Live + backup

Use `config.sample-backup.json` or set `CLICKHOUSE_BACKUP_SERVERS`:

```bash
docker run -d --name clickhouse-bulk \
  -p 8124:8124 \
  -v "$(pwd)/config.sample-backup.json:/app/config.json:ro" \
  -v bulk-dumps:/app/dumps \
  -v bulk-dumps-bkp:/app/dumps-bkp \
  -v bulk-journal:/app/journal \
  itcrow/clickhouse-bulk:latest \
  ./clickhouse-bulk -config=/app/config.json
```

---

## Volumes

| Path in container | Purpose |
|-------------------|---------|
| `/app/config.json` | Config (mount read-only) |
| `/app/dumps` | Live failed-batch replay (`dump_dir`) |
| `/app/dumps-bkp` | Backup dumps (`bkp_dump_dir`, dual-write) |
| `/app/journal` | WAL (`journal_dir`) |

Use named volumes or bind mounts so data survives container restarts.

---

## Docker Compose (minimal)

```yaml
services:
  clickhouse-bulk:
    image: itcrow/clickhouse-bulk:latest
    ports:
      - "8124:8124"
    volumes:
      - ./config.json:/app/config.json:ro
      - bulk-dumps:/app/dumps
      - bulk-journal:/app/journal
    command: ["./clickhouse-bulk", "-config=/app/config.json"]
    environment:
      CLICKHOUSE_SERVERS: http://clickhouse:8123
    depends_on:
      - clickhouse

  clickhouse:
    image: clickhouse/clickhouse-server:24
    ports:
      - "8123:8123"

volumes:
  bulk-dumps:
  bulk-journal:
```

---

## Build and push (maintainers)

Local build from [Dockerfile](../Dockerfile):

```bash
docker build -t itcrow/clickhouse-bulk:tagname .

docker push itcrow/clickhouse-bulk:tagname
```

Example after a release tag:

```bash
docker pull itcrow/clickhouse-bulk:1.0.0
docker tag itcrow/clickhouse-bulk:1.0.0 itcrow/clickhouse-bulk:latest
docker push itcrow/clickhouse-bulk:latest   # only if you own the repo
```

Official releases: push a semver git tag; CI publishes `itcrow/clickhouse-bulk:<tag>` and `latest` via GoReleaser (requires `DOCKERHUB_TOKEN` — see [README](../README.md#docker-hub)).

Makefile shortcut:

```bash
make docker_build
docker tag clickhouse-bulk itcrow/clickhouse-bulk:tagname
docker push itcrow/clickhouse-bulk:tagname
```

---

## Image details

- Base runtime: `alpine:3` + `ca-certificates`
- Exposed port: **8124** (bulk HTTP; config `listen`)
- Default command: `./clickhouse-bulk -config=config.sample.json`
- GoReleaser image: [Dockerfile.goreleaser](../Dockerfile.goreleaser) (binary only, same layout)

---

## Related

- [CONFIG.md](./CONFIG.md) — all settings and env vars
- [DUAL_WRITE.md](./DUAL_WRITE.md) — live + backup
- [CLIENT_COMPATIBILITY.md](./CLIENT_COMPATIBILITY.md) — point apps at `:8124`, not CH `:8123` for batched ingest
