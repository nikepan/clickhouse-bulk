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

const qFormatInQuotesQuery = "INSERT INTO test (date, args) VALUES"
const qFormatInQuotesValues = "('2019-06-13', 'query=select%20args%20from%20test%20group%20by%20date%20FORMAT%20JSON')"

const qTSNamesTitle = "INSERT INTO table3 (c1, c2, c3) FORMAT TabSeparatedWithNames"
const qNames = "field1	field2	field3"

var escTitle = url.QueryEscape(qTitle)
var escSelect = url.QueryEscape(qSelect)
var escParamsAndSelect = qParams + "&query=" + escSelect

func BenchmarkCollector_Push(t *testing.B) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	for i := 0; i < 30000; i++ {
		c.Push(escTitle, qContent)
	}
}

func TestCollector_Push(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	for i := 0; i < 10400; i++ {
		c.Push(escTitle, qContent)
	}
	assert.Equal(t, 400, c.Tables[escTitle].GetCount())
}

func BenchmarkCollector_ParseQuery(b *testing.B) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	c.ParseQuery("", qTitle+" "+qContent)
	c.ParseQuery(qParams, qTitle+" "+qContent)
	c.ParseQuery("query="+escTitle, qContent)
	c.ParseQuery(qParams+"&query="+escTitle, qContent)
}

func TestCollector_ParseQuery(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
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
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	query, params := c.separateQuery(escParamsAndSelect)
	assert.Equal(t, qSelect, query)
	assert.Equal(t, qParams, params)
}

func TestTable_getFormat(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	f := c.getFormat(qTitle)
	assert.Equal(t, "TabSeparated", f)
}

func TestTable_CheckFlush(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	c.Push(qTitle, qContent)
	count := 0
	for !c.Tables[qTitle].Empty() {
		time.Sleep(time.Millisecond * time.Duration(100))
		count++
	}
	assert.True(t, count >= 9)
}

func TestCollector_FlushAll(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	c.Push(qTitle, qContent)
	c.FlushAll()
}

func TestHandleExistingIndices(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		i           int
		wantPrefix  string
		wantContent string
	}{
		{
			name:        "test0",
			text:        qTitle,
			i:           0,
			wantPrefix:  "INSERT INTO ",
			wantContent: "table3 (c1, c2, c3) FORMAT TabSeparated",
		},
		{
			name:        "test1",
			text:        qTitle,
			i:           1,
			wantPrefix:  "INSERT INTO ",
			wantContent: "table3 (c1, c2, c3) FORMAT TabSeparated",
		},
		{
			name:        "test2",
			text:        qTitle,
			i:           4,
			wantPrefix:  "INSERT INTO table3 ",
			wantContent: "(c1, c2, c3) FORMAT TabSeparated",
		},
		{
			name:        "test3",
			text:        qTitle,
			i:           5,
			wantPrefix:  "INSERT INTO table3 ",
			wantContent: "(c1, c2, c3) FORMAT TabSeparated",
		},
		{
			name:        "test4",
			text:        qTitle + " " + qContent,
			i:           3,
			wantPrefix:  "INSERT INTO ",
			wantContent: "table3 (c1, c2, c3) FORMAT TabSeparated v11\tv12\tv13\nv21\tv22\tv23",
		},
		{
			name:        "test5",
			text:        qTitle + " " + qContent,
			i:           4,
			wantPrefix:  "INSERT INTO table3 ",
			wantContent: "(c1, c2, c3) FORMAT TabSeparated v11\tv12\tv13\nv21\tv22\tv23",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
			gotPrefix, gotContent := c.handleExistingIndices(tt.text, tt.i)
			assert.Equal(t, tt.wantPrefix, gotPrefix)
			assert.Equal(t, tt.wantContent, gotContent)
		})
	}
}

func TestCleanTable(t *testing.T) {
	c := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	c.AddTable("test")
	table := c.Tables["test"]
	table.Add(qTitle + " " + qContent)

	c.CleanTables()

	select {
	case _, ok := <-*table.TickerChan:
		if ok {
			t.Fatal("Table channel is not closed")
		}
	default:
		t.Fatal("Table channel is not closed")
	}

	assert.Equal(t, true, c.Empty())

}
