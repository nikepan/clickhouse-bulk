package main

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const formatValues = "values"
const formatTabSeparated = "tabseparated"

var regexFormat = regexp.MustCompile("(?i)format\\s\\S+(\\s+)")
var regexValues = regexp.MustCompile("(?i)\\svalues\\s")
var regexGetFormat = regexp.MustCompile("(?i)format\\s(\\S+)")

// Table - store query table info
type Table struct {
	Name          string
	Format        string
	Query         string
	Params        string
	Rows          []string
	count         int
	FlushCount    int
	FlushInterval int
	mu            sync.Mutex
	Sender        Sender
	// todo add Last Error
}

// Collector - query collector
type Collector struct {
	Tables        map[string]*Table
	mu            sync.RWMutex
	Count         int
	FlushInterval int
	Sender        Sender
}

type FormatType int8

const (
	Unknown FormatType = iota
	SqlValues
	JSONEachRow
	TabSeparated
)

// NewTable - default table constructor
func NewTable(name string, sender Sender, count int, interval int) (t *Table) {
	t = new(Table)
	t.Name = name
	t.Sender = sender
	t.FlushCount = count
	t.FlushInterval = interval
	return t
}

// NewCollector - default collector constructor
func NewCollector(sender Sender, count int, interval int) (c *Collector) {
	c = new(Collector)
	c.Sender = sender
	c.Tables = make(map[string]*Table)
	c.Count = count
	c.FlushInterval = interval
	return c
}

// Content - get text content of rowsfor query
func (t *Table) Content() string {
	return t.Query + "\n" + strings.Join(t.Rows, "\n")
}

// Flush - sends collected data in table to clickhouse
func (t *Table) Flush() {
	req := ClickhouseRequest{
		Params:  t.Params,
		Query:   t.Query,
		Content: t.Content(),
		Count:   t.count,
	}
	t.Sender.Send(&req)
	t.Rows = make([]string, 0, t.FlushCount)
	t.count = 0
}

// CheckFlush - check if flush is need and sends data to clickhouse
func (t *Table) CheckFlush() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count > 0 {
		t.Flush()
		return true
	}
	return false
}

// Empty - Checks if table is empty
func (t *Table) Empty() bool {
	return t.GetCount() == 0
}

// GetCount - Checks if table is empty
func (t *Table) GetCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}

// RunTimer - timer for periodical savings data
func (t *Table) RunTimer() {
	ticker := time.NewTicker(time.Millisecond * time.Duration(t.FlushInterval))
	go func() {
		for range ticker.C {
			t.CheckFlush()
		}
	}()
}

// Add - Adding query to table
func (t *Table) Add(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count++
	t.Rows = append(t.Rows, text)
	if len(t.Rows) >= t.FlushCount {
		t.Flush()
	}
}

// Empty - check if all tables are empty
func (c *Collector) Empty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range c.Tables {
		if ok := t.Empty(); !ok {
			return false
		}
	}
	return true
}

// FlushAll - flush all tables to clickhouse
func (c *Collector) FlushAll() (count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	count = 0
	for _, t := range c.Tables {
		if ok := t.CheckFlush(); ok {
			count++
		}

	}
	return count
}

// WaitFlush - wait for flush all tables
func (c *Collector) WaitFlush() (err error) {
	return c.Sender.WaitFlush()
}

// AddTable - adding table to collector
func (c *Collector) AddTable(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addTable(name)
}

func (c *Collector) separateQuery(name string) (query string, params string) {
	items := strings.Split(name, "&")
	for _, p := range items {
		if HasPrefix(p, "query=") {
			query = p[6:]
		} else {
			params += "&" + p
		}
	}
	if len(params) > 0 {
		params = strings.TrimSpace(params[1:])
	}
	q, err := url.QueryUnescape(query)
	if err != nil {
		return "", name
	}
	return q, params
}

func (c *Collector) getFormat(query string) (format string) {
	format = formatValues
	f := regexGetFormat.FindSubmatch([]byte(query))
	if len(f) > 1 {
		format = strings.TrimSpace(string(f[1]))
	}
	return format
}

func (c *Collector) addTable(name string) *Table {
	t := NewTable(name, c.Sender, c.Count, c.FlushInterval)
	query, params := c.separateQuery(name)
	t.Query = query
	t.Params = params
	t.Format = c.getFormat(query)
	c.Tables[name] = t
	t.RunTimer()
	return t
}

// Push - adding query to collector with query params (with query) and rows
func (c *Collector) Push(params string, content string) {
	c.mu.RLock()
	table, ok := c.Tables[params]
	if ok {
		table.Add(content)
		c.mu.RUnlock()
		pushCounter.Inc()
		return
	}
	c.mu.RUnlock()
	c.mu.Lock()
	table, ok = c.Tables[params]
	if !ok {
		table = c.addTable(params)
	}
	table.Add(content)
	c.mu.Unlock()
	pushCounter.Inc()
}

// ParseQuery - parsing inbound query to unified format (params/query), content (query data)
func (c *Collector) ParseQuery(queryString string, body string) (params string, content string, insert bool) {
	i := strings.Index(queryString, "query=")
	if i >= 0 {
		if HasPrefix(queryString[i+6:], "insert") {
			insert = true
		}
		var q string
		eoq := strings.Index(queryString[i+6:], "&")
		if eoq >= 0 {
			q = queryString[i+6 : eoq+6]
			params = queryString[:i] + queryString[eoq+7:]
		} else {
			q = queryString[i+6:]
			params = queryString[:i]
		}
		uq, err := url.QueryUnescape(q)
		if body != "" {
			uq += " " + body
		}
		if err != nil {
			return queryString, body, false
		}
		prefix, cnt := c.Parse(uq)
		if strings.HasSuffix(params, "&") || params == "" {
			params += "query=" + url.QueryEscape(strings.TrimSpace(prefix))
		} else {
			params += "&query=" + url.QueryEscape(strings.TrimSpace(prefix))
		}
		content = cnt
	} else {
		var q string
		q, content = c.Parse(body)
		q = strings.TrimSpace(q)
		if HasPrefix(q, "insert") {
			insert = true
		}
		if queryString != "" {
			params = queryString + "&query=" + url.QueryEscape(q)
		} else {
			params = "query=" + url.QueryEscape(q)
		}
	}
	return strings.TrimSpace(params), strings.TrimSpace(content), insert
}

func GetInsertFormatType(query string) (format FormatType, dataPos int) {
	k := strings.Index(query, "VALUES")
	if k != -1 {
		return SqlValues, k + len("VALUES")
	}
	k = strings.Index(query, "FORMAT")
	if k != -1 {
		k = k + 7
		// todo check length
		formatSubString := strings.TrimLeft(query[k:k+20], "\t \n")
		if strings.HasPrefix(formatSubString, "JSONEachRow") {
			return JSONEachRow, k + len("JSONEachRow")
		}
		if strings.HasPrefix(formatSubString, "TabSeparated") {
			return TabSeparated, k + len("TabSeparated")
		}
	}
	return Unknown, -1
}

// Parse - parsing text for query and data
func (c *Collector) Parse(text string) (prefix string, content string) {
	formatType, dataPos := GetInsertFormatType(text)

	switch formatType {
	case SqlValues, JSONEachRow, TabSeparated:
		prefix = strings.TrimSpace(text[:dataPos])
		content = strings.TrimSpace(text[dataPos:])
	case Unknown:
		// TODO undefined
		prefix = text
		content = ""
	}
	return prefix, content
}
