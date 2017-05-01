/*

ClickHouse-Bulk

Simple Yandex ClickHouse (https://clickhouse.yandex/) insert collector. It collect requests and send to ClickHouse servers.


Features

- Group n requests and send to any of ClickHouse server

- Sending collected data by interval

- Tested with VALUES, TabSeparated formats

- Supports many servers to send

- Supports query in query parameters and in body

- Supports other query parameters like username, password, database


For example:

INSERT INTO table3 (c1, c2, c3) VALUES ('v1', 'v2', 'v3')

INSERT INTO table3 (c1, c2, c3) VALUES ('v4', 'v5', 'v6')

sends as

INSERT INTO table3 (c1, c2, c3) VALUES ('v1', 'v2', 'v3')('v4', 'v5', 'v6')


Options

- -config - config file (json); default _config.json_


Configuration file

{
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


Quickstart

`./clickhouse-bulk`
and send queries to :8124


Tips

For better performance words FORMAT and VALUES must be uppercase.

*/
package main
