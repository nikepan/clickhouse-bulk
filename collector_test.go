package main

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const qTitle = "INSERT INTO table3 (c1, c2, c3) FORMAT TabSeparated"
const qContent = "v11	v12	v13\nv21	v22	v23"
const qValuesTitle = "INSERT INTO table3 (c1, c2, c3) Values"
const qValuesTitleUpper = "INSERT INTO table3 (c1, c2, c3) VALUES"
const qValuesContent = "(v11,v12,v13),(v21,v22,v23)"
const qSelect = "SELECT 1"
const qParams = "user=user&password=111"
const qSelectAndParams = "query=" + qSelect + "&" + qParams
const badEscQuery = "query=INSERT %zdwfr"

var escTitle = url.QueryEscape(qTitle)
var escSelect = url.QueryEscape(qSelect)
var escParamsAndSelect = qParams + "&query=" + escSelect

var qFormatInQuotesQuery = "INSERT INTO test (date, args) VALUES"
var qFormatInQuotesValues = "('2019-06-13', 'query=select%20args%20from%20test%20group%20by%20date%20FORMAT%20JSON')"

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
	assert.Equal(t, 400, c.Tables[escTitle].GetCount())
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

	assert.Equal(t, "query="+escTitle, params)
	assert.Equal(t, qContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery(qParams, qTitle+" "+qContent)

	assert.Equal(t, qParams+"&query="+escTitle, params)
	assert.Equal(t, qContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery("query="+escTitle, qContent)

	assert.Equal(t, "query="+escTitle, params)
	assert.Equal(t, qContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery(qParams+"&query="+escTitle, qContent)

	assert.Equal(t, qParams+"&query="+escTitle, params)
	assert.Equal(t, qContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery("query="+escSelect, "")

	assert.Equal(t, "query="+escSelect, params)
	assert.Equal(t, "", content)
	assert.Equal(t, false, insert)

	params, content, insert = c.ParseQuery(qSelectAndParams, "")
	assert.Equal(t, escParamsAndSelect, params)
	assert.Equal(t, "", content)
	assert.Equal(t, false, insert)

	params, content, insert = c.ParseQuery("query="+url.QueryEscape(qValuesTitle+" "+qValuesContent), "")

	assert.Equal(t, "query="+url.QueryEscape(qValuesTitle), params)
	assert.Equal(t, qValuesContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery("", qSelect)

	assert.Equal(t, "query="+escSelect, params)
	assert.Equal(t, "", content)
	assert.Equal(t, false, insert)

	params, content, insert = c.ParseQuery("", strings.ToLower(qTitle)+" "+qContent)

	assert.Equal(t, "query="+strings.ToLower(escTitle), strings.ToLower(params))
	assert.Equal(t, qContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery("", strings.ToLower(qValuesTitle)+" "+qValuesContent)

	assert.Equal(t, "query="+strings.ToLower(url.QueryEscape(qValuesTitle)), strings.ToLower(params))
	assert.Equal(t, qValuesContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery("", qValuesTitleUpper+" "+qValuesContent)

	assert.Equal(t, "query="+strings.ToLower(url.QueryEscape(qValuesTitleUpper)), strings.ToLower(params))
	assert.Equal(t, qValuesContent, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery(badEscQuery, qValuesTitleUpper+" "+qValuesContent)

	assert.False(t, insert)

	params, content, insert = c.ParseQuery("", qFormatInQuotesQuery+" "+qFormatInQuotesValues)
	assert.Equal(t, "query="+url.QueryEscape(qFormatInQuotesQuery), params)
	assert.Equal(t, qFormatInQuotesValues, content)
	assert.Equal(t, true, insert)

	params, content, insert = c.ParseQuery("query="+url.QueryEscape(qFormatInQuotesQuery+" "+qFormatInQuotesValues), "")
	assert.Equal(t, "query="+url.QueryEscape(qFormatInQuotesQuery), params)
	assert.Equal(t, qFormatInQuotesValues, content)
	assert.Equal(t, true, insert)
}

func TestCollector_separateQuery(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	query, params := c.separateQuery(escParamsAndSelect)
	assert.Equal(t, qSelect, query)
	assert.Equal(t, qParams, params)
}

func TestTable_getFormat(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	f := c.getFormat(qTitle)
	assert.Equal(t, "TabSeparated", f)
}

func TestTable_CheckFlush(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	c.Push(qTitle, qContent)
	count := 0
	for !c.Tables[qTitle].Empty() {
		time.Sleep(time.Millisecond * time.Duration(100))
		count++
	}
	assert.True(t, count >= 9)
}

func TestCollector_FlushAll(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000)
	c.Push(qTitle, qContent)
	c.FlushAll()
}
