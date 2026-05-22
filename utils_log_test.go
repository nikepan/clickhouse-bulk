package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactURLParams(t *testing.T) {
	assert.Equal(t, "", redactURLParams(""))
	assert.Equal(t, "user=default&password=***", redactURLParams("user=default&password=secret"))
	assert.Equal(t, "query=INSERT&token=***", redactURLParams("query=INSERT&token=abc123"))
	assert.Equal(t, "database=db", redactURLParams("database=db"))
}

func TestLogInsertMeta(t *testing.T) {
	meta := logInsertMeta("user=u&password=***", "a\nb\nc")
	assert.Contains(t, meta, "params=")
	assert.Contains(t, meta, "rows=3")
	assert.Contains(t, meta, "content_bytes=5")
	assert.NotContains(t, meta, "password=secret")
}
