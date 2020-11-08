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
	TickerChan    *chan struct{}
	lastUpdate    time.Time
	// todo add Last Error
}

// Collector - query collector
type Collector struct {
	Tables        map[string]*Table
	mu            sync.RWMutex
	Count         int
	FlushInterval int
	CleanInterval int
	RemoveQueryID bool
	Sender        Sender
	TickerChan    *chan struct{}
}

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
func NewCollector(sender Sender, count int, interval int, cleanInterval int, removeQueryID bool) (c *Collector) {
	c = new(Collector)
	c.Sender = sender
	c.Tables = make(map[string]*Table)
	c.Count = count
	c.FlushInterval = interval
	c.CleanInterval = cleanInterval
	c.RemoveQueryID = removeQueryID
	if cleanInterval > 0 {
		c.TickerChan = c.RunTimer()
	}
	return c
}

// Content - get text content of rowsfor query
func (t *Table) Content() string {
	rowDelimiter := "\n"
	if t.Format == "RowBinary" {
		rowDelimiter = ""
	}
	return t.Query + "\n" + strings.Join(t.Rows, rowDelimiter)
}

// Flush - sends collected data in table to clickhouse
func (t *Table) Flush() {
	req := ClickhouseRequest{
		Params:   t.Params,
		Query:    t.Query,
		Content:  t.Content(),
		Count:    len(t.Rows),
		isInsert: true,
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
func (t *Table) RunTimer() *chan struct{} {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond * time.Duration(t.FlushInterval))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.CheckFlush()
			case <-done:
				return
			}
		}
	}()
	return &done
}

// Add - Adding query to table
func (t *Table) Add(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count++
	if t.Format == "TabSeparated" {
		t.Rows = append(t.Rows, strings.Split(text, "\n")...)
	} else {
		t.Rows = append(t.Rows, text)
	}
	if len(t.Rows) >= t.FlushCount {
		t.Flush()
	}
	t.lastUpdate = time.Now()
}

// CleanTable - delete table from map
func (t *Table) CleanTable() {
	t.mu.Lock()
	close(*t.TickerChan)
	t = nil
}

// CleanTables - clean unsused tables
func (c *Collector) CleanTables() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, t := range c.Tables {
		if t.lastUpdate.Add(time.Duration(c.CleanInterval) * time.Millisecond).Before(time.Now()) {
			// table was not updated for CleanInterval - delete that table - otherwise it can cause memLeak
			t.CleanTable()
			defer delete(c.Tables, k)
		}

	}
}

// RunTimer - timer for periodical cleaning unused tables
func (c *Collector) RunTimer() *chan struct{} {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Duration(c.CleanInterval) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.CleanTables()
			case <-done:
				return
			}
		}
	}()
	return &done
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
	t.TickerChan = t.RunTimer()
	t.lastUpdate = time.Now()
	return t
}

// Push - adding query to collector with query params (with query) and rows
func (c *Collector) Push(paramsIn string, content string) {
	// as we are using all params a table key, we have to remove query_id
	params := ""
	if c.RemoveQueryID {
		items := strings.Split(paramsIn, "&")
		for _, p := range items {
			if !HasPrefix(p, "query_id=") {
				//params = strings.ReplaceAll(params, p, "")
				params += "&" + p
			}
		}
		if len(params) > 0 {
			params = strings.TrimSpace(params[1:])
		}
	} else {
		params = paramsIn
	}
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

// Parse - parsing text for query and data
func (c *Collector) Parse(text string) (prefix string, content string) {
	i := strings.Index(text, "FORMAT")
	k := strings.Index(text, "VALUES")
	if k == -1 {
		k = strings.Index(text, "values")
	}
	if i >= 0 && i < k {
		w := false
		off := -1
		for c := i + 7; c < len(text); c++ {
			if !w && text[c] != ' ' && text[c] != '\n' && text[c] != ';' {
				w = true
			}
			if w && (text[c] == ' ' || text[c] == '\n' || text[c] == ';') {
				off = c + 1
				break
			}
		}
		if off >= 0 {
			prefix = text[:off]
			content = text[off:]
		}
	} else {
		if k >= 0 {
			prefix = strings.TrimSpace(text[:k+6])
			content = strings.TrimSpace(text[k+6:])
		} else {
			off := regexFormat.FindStringSubmatchIndex(text)
			if len(off) > 3 {
				prefix = text[:off[3]]
				content = text[off[3]:]
			} else {
				off := regexValues.FindStringSubmatchIndex(text)
				if len(off) > 0 {
					prefix = text[:off[1]]
					content = text[off[1]:]
				} else {
					prefix = text
				}
			}
		}
	}
	return prefix, content
}
