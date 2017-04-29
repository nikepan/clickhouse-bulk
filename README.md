# Clickhouse-bulk
 
[![Build Status](https://travis-ci.org/nikepan/clickhouse-bulk.svg?branch=master)](https://travis-ci.org/nikepan/clickhouse-bulk)
[![codecov](https://codecov.io/gh/nikepan/clickhouse-bulk/branch/master/graph/badge.svg)](https://codecov.io/gh/nikepan/clickhouse-bulk)
[![Go Report Card](https://goreportcard.com/badge/github.com/nikepan/clickhouse-bulk)](https://goreportcard.com/report/github.com/nikepan/clickhouse-bulk)
[![godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/nikepan/clickhouse-bulk)


Simple [Yandex ClickHouse](https://clickhouse.yandex/) insert collector. It collect requests and send to ClickHouse servers.


### Installation from sources:
```text
git clone github.com/nikepan/clickhouse-bulk
cd clickhouse-bulk
go get
go build
```


### Features
- Group n requests and send to any of ClickHouse server
- Sending collected data by interval
- Tested with VALUES, TabSeparated formats

For example:
```sql
INSERT INTO table3 (c1, c2, c3) VALUES ('v1', 'v2', 'v3')
INSERT INTO table3 (c1, c2, c3) VALUES ('v4', 'v5', 'v6')
```
sends as
```sql
INSERT INTO table3 (c1, c2, c3) VALUES ('v1', 'v2', 'v3')('v4', 'v5', 'v6')
```


### Options
- -config - config file (json); default _config.json_


### Configuration file
```json{
  "listen": ":8124", 
  "flush_count": 10000, // check by \n char
  "flush_interval": 1000, // milliseconds
  "debug": false, // log incoming requests
  "clickhouse": {
    "down_timeout": 300, // wait if server in down (seconds)
    "servers": [
      "http://127.0.0.1:8123"
    ]
  }
}
```


### Quickstart

`./clickhouse-bulk`
and send queries to :8124


### Tips

For better performance words FORMAT and VALUES must be uppercase.