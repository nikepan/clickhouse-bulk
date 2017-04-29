package main

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"strings"
	"testing"
	"time"
)

const qTitle = "INSERT INTO table3 (c1, c2, c3) FORMAT TabSeparated"
const qContent = "v11	v12	v13\nv21	v22	v23"
const qValuesTitle = "INSERT INTO table3 (c1, c2, c3) Values"
const qValuesTitleUpper = "INSERT INTO table3 (c1, c2, c3) VALUES"
const qValuesContent = "(v11,v12,v13),(v21,v22,v23)"
const qSelect = "SELECT 1"
const qParams = "user=user&password=111"

var escTitle = url.QueryEscape(qTitle)
var escSelect = url.QueryEscape(qSelect)

func BenchmarkCollector_Push(t *testing.B) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	for i := 0; i < 30000; i++ {
		c.Push(escTitle, qContent)
	}
}

func TestCollector_Push(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	for i := 0; i < 10400; i++ {
		c.Push(escTitle, qContent)
	}
	assert.Equal(t, c.Tables[escTitle].Count, 800)
}

func BenchmarkCollector_ParseQuery(b *testing.B) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	c.ParseQuery("", qTitle+" "+qContent)
	c.ParseQuery(qParams, qTitle+" "+qContent)
	c.ParseQuery("query="+escTitle, qContent)
	c.ParseQuery(qParams+"&query="+escTitle, qContent)
}

func TestCollector_ParseQuery(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	var params string
	var content string
	var insert bool

	params, content, insert = c.ParseQuery("", qTitle+" "+qContent)

	assert.Equal(t, params, "query="+escTitle)
	assert.Equal(t, content, qContent)
	assert.Equal(t, insert, true)

	params, content, insert = c.ParseQuery(qParams, qTitle+" "+qContent)

	assert.Equal(t, params, qParams+"&query="+escTitle)
	assert.Equal(t, content, qContent)
	assert.Equal(t, insert, true)

	params, content, insert = c.ParseQuery("query="+escTitle, qContent)

	assert.Equal(t, params, "query="+escTitle)
	assert.Equal(t, content, qContent)
	assert.Equal(t, insert, true)

	params, content, insert = c.ParseQuery(qParams+"&query="+escTitle, qContent)

	assert.Equal(t, params, qParams+"&query="+escTitle)
	assert.Equal(t, content, qContent)
	assert.Equal(t, insert, true)

	params, content, insert = c.ParseQuery("query="+escSelect, "")

	assert.Equal(t, params, "query="+escSelect)
	assert.Equal(t, content, "")
	assert.Equal(t, insert, false)

	params, content, insert = c.ParseQuery("", qSelect)

	assert.Equal(t, params, "query="+escSelect)
	assert.Equal(t, content, "")
	assert.Equal(t, insert, false)

	params, content, insert = c.ParseQuery("", strings.ToLower(qTitle)+" "+qContent)

	assert.Equal(t, strings.ToLower(params), "query="+strings.ToLower(escTitle))
	assert.Equal(t, content, qContent)
	assert.Equal(t, insert, true)

	params, content, insert = c.ParseQuery("", strings.ToLower(qValuesTitle)+" "+qValuesContent)

	assert.Equal(t, strings.ToLower(params), "query="+strings.ToLower(url.QueryEscape(qValuesTitle)))
	assert.Equal(t, content, qValuesContent)
	assert.Equal(t, insert, true)

	params, content, insert = c.ParseQuery("", qValuesTitleUpper+" "+qValuesContent)

	assert.Equal(t, strings.ToLower(params), "query="+strings.ToLower(url.QueryEscape(qValuesTitleUpper)))
	assert.Equal(t, content, qValuesContent)
	assert.Equal(t, insert, true)
}

func TestTable_CheckFlush(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1)
	c.Push(qTitle, qContent)
	for !c.Tables[qTitle].Empty() {
		time.Sleep(10)
	}
}

func TestCollector_FlushAll(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	c.Push(qTitle, qContent)
	c.FlushAll()
}
