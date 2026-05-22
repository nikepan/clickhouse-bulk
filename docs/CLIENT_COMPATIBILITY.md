# Client compatibility (HTTP drivers)

clickhouse-bulk is an **INSERT batching proxy** in front of ClickHouse’s [HTTP interface](https://clickhouse.com/docs/interfaces/http). It is **not** a drop-in replacement for official language clients.

This document describes **current** behaviour and **planned** improvements. See [Plan.md](../Plan.md#p4--client-compatibility-optional) for the implementation roadmap.

---

## How bulk handles HTTP (baseline)

| Path | Detection | Client response | To ClickHouse |
|------|-----------|-----------------|---------------|
| **Batched INSERT** | `INSERT` in `query=` or body | `200` + **empty body** (async) | POST `text/plain` after flush |
| **Proxied query** | Not `INSERT` | CH **status + body** (sync) | POST `text/plain`, same request |
| **Dual-write** | Batched INSERT only | Same async `200` | Live + backup queues |

Important differences from direct `:8123`:

- No forwarding of `X-ClickHouse-*` response headers to the client.
- No request decompression (`Content-Encoding: lz4`, `gzip`, …).
- Outbound POST to CH uses `Content-Type: text/plain` only.
- `remove_query_id: true` (default) strips `query_id` from the batching key.
- With `journal_dir`, HTTP `200` means **WAL append**, not “row visible in CH”.

---

## Summary matrix

| Client | Transport to bulk | Typical use with bulk | Works today? |
|--------|-------------------|------------------------|--------------|
| [clickhouse-go](https://github.com/ClickHouse/clickhouse-go) v2 | Native TCP `:9000` | Point driver at **ClickHouse**, not bulk | ✅ (bypass bulk) |
| clickhouse-go v2 | HTTP `:8124` | Batch API / `PrepareBatch` | ❌ Not recommended |
| [clickhouse-connect](https://clickhouse.com/docs/integrations/python) | HTTP `:8124` | `query()` / `command()` | ✅ Good |
| clickhouse-connect | HTTP `:8124` | `raw_insert` text formats, `compress=False` | ⚠️ Partial |
| clickhouse-connect | HTTP `:8124` | `insert()` (Native default) | ❌ Poor |
| curl / Vector / custom TSV | HTTP `:8124` | `INSERT … FORMAT TabSeparated` | ✅ **Intended** |

---

## clickhouse-go (v2)

Official driver: [ClickHouse/clickhouse-go](https://github.com/ClickHouse/clickhouse-go). Supports **native TCP** (`:9000`) and **HTTP** (`:8123`) with **Native binary encoding** on both transports.

### What the driver sends over HTTP

- `PrepareBatch` / batch insert → `INSERT INTO <table> FORMAT Native` + `application/octet-stream` body.
- Optional **LZ4 / ZSTD / gzip** (`Compression` in `clickhouse.Options`).
- Expects a **synchronous** CH response; reads result body to reuse the connection.
- `Query()` / `Exec()` for non-insert SQL → standard HTTP query.

### Compatibility with bulk

| API | Via bulk (`:8124`) | Notes |
|-----|-------------------|--------|
| `clickhouse.Open` / `OpenDB`, **Protocol: Native** (default) | N/A | Connect to ClickHouse directly — **best for Go apps** |
| `Protocol: clickhouse.HTTP` + `PrepareBatch` / `AsyncInsert` | ❌ | Native binary + compression; bulk parses **text** INSERT only |
| `Protocol: clickhouse.HTTP` + `Query` / `Exec` (SELECT, DDL) | ✅ | Sync proxy on **live**; backup not used |
| `database/sql` over HTTP DSN | ❌ for `INSERT`; ✅ for reads | Same as above |

### Example (recommended split)

```go
// Ingest high-volume batches through bulk (if you build HTTP yourself) or separate service.
// Application writes: use native TCP to ClickHouse.
conn, err := clickhouse.Open(&clickhouse.Options{
    Addr: []string{"clickhouse:9000"},
    Auth: clickhouse.Auth{Database: "db", Username: "user", Password: "pass"},
    // Protocol: Native (default)
})
```

```go
// HTTP only when you must — point at ClickHouse, not bulk, for clickhouse-go.
conn := clickhouse.OpenDB(&clickhouse.Options{
    Addr:     []string{"clickhouse:8123"},
    Protocol: clickhouse.HTTP,
    Auth:     clickhouse.Auth{Database: "db", Username: "user", Password: "pass"},
})
```

### Dual-write note

Only **batched INSERT** traffic through bulk is copied to backup. Go apps using **native TCP to live** do not automatically populate standby unless another path feeds backup.

---

## Python clickhouse-connect

Official driver: [clickhouse-connect](https://clickhouse.com/docs/integrations/python). Uses **HTTP(S)** only ([driver API](https://clickhouse.com/docs/integrations/language-clients/python/driver-api)).

### What the driver sends

| Method | Payload | Response expectations |
|--------|---------|------------------------|
| `client.insert()` | `INSERT … FORMAT Native` (default), optional compression | `QuerySummary`; uses `X-ClickHouse-Summary` / `X-ClickHouse-Query-Id` headers; **raises on failure** |
| `client.raw_insert()` | Caller-chosen `fmt` (e.g. `TabSeparated`) | Same |
| `client.query()` / `command()` | SQL | Full CH response |

### Compatibility with bulk

| API | Via bulk | Notes |
|-----|----------|--------|
| `get_client(host=bulk, port=8124)` + `query()` / `command()` | ✅ | Same as direct HTTP for non-INSERT |
| `raw_insert(..., fmt='TabSeparated', compression=None)` | ⚠️ | Text INSERT can be batched; **no CH error returned** to Python; no summary headers |
| `insert()` / `insert_df()` / Arrow | ❌ | Native/binary; bulk cannot merge batches; async `200` hides CH errors |
| `compress=True` (default possible) | ❌ | Bulk does not decompress request body |

### Example (partial compatibility)

```python
import clickhouse_connect

client = clickhouse_connect.get_client(
    host="bulk-host",
    port=8124,
    database="mydb",
    username="default",
    password="secret",
    compress=False,
)

# OK — proxied to live ClickHouse
print(client.query("SELECT count() FROM events").result_rows)

# Possible — understand async semantics; errors only in bulk logs/metrics
client.raw_insert(
    table="events",
    column_names=["id", "val"],
    insert_block="1\tfoo\n2\tbar\n",
    fmt="TabSeparated",
    compression=None,
)

# Not recommended through bulk
# client.insert("events", [[1, "a"], [2, "b"]], column_names=["id", "val"])
```

### Recommended Python layout

| Workload | Target |
|----------|--------|
| App reads / DDL / low-rate typed inserts | `clickhouse-connect` → **ClickHouse `:8123`** |
| High-volume fire-and-forget TSV/JSONEachRow | Agents → **bulk `:8124`** |
| Need insert exceptions in Python | **ClickHouse directly**, not bulk |

---

## Comparison: bulk vs direct HTTP to ClickHouse

| Feature | Direct CH `:8123` | bulk `:8124` |
|---------|-------------------|--------------|
| INSERT batching across clients | No (per request) | Yes (by params key) |
| INSERT response | CH body (`Ok.`, errors) | Empty `200` (queued) |
| Response headers | Full | Not forwarded (insert); proxied queries: body only |
| Request compression | Supported | Not supported |
| Native / octet-stream INSERT | Supported | Not parsed (text INSERT path only) |
| Journal before accept | No | Optional (`journal_dir`) |
| Dual-write to standby | No (use CH replication) | Optional (`clickhouse-backup`) |

---

## Planned improvements (roadmap)

See [Plan.md — P4 Client compatibility](../Plan.md#p4--client-compatibility-optional). Short overview:

| Phase | Feature | Unlocks |
|-------|---------|---------|
| **P4.1** | Opaque INSERT passthrough (Native / octet-stream, no batch merge) | clickhouse-go HTTP batch, connect `insert()` payload pass-through (still async) |
| **P4.2** | Request decompression; preserve `Content-Type` to CH | LZ4/gzip clients |
| **P4.3** | Forward `X-ClickHouse-*` on proxied queries; optional on passthrough | Richer `QuerySummary` where CH returns headers |
| **P4.4** | Config: `batch_formats` vs passthrough formats | Hybrid: batch TSV, passthrough Native |
| **P4.5** | Optional `sync_insert` / `X-Bulk-Sync: 1` | Driver-like error propagation (throughput cost) |

**Non-goals:** replacing clickhouse-go/connect; batch-merging Native blocks; full transparent proxy without mode flags.

---

## Related docs

- [DUAL_WRITE.md](./DUAL_WRITE.md) — architecture, journal, dumps
- [RISKS.md](./RISKS.md) — operational risks (async `200`, backup lag)
- [CONFIG.md](./CONFIG.md) — `remove_query_id`, `journal_dir`, dual-write
