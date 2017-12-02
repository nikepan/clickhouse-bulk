package main

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var regexFormat = regexp.MustCompile("(?i)format\\s\\S+(\\s+)")
var regexValues = regexp.MustCompile("(?i)\\svalues\\s")

// Table - store query table info
type Table struct {
	Name          string
	Rows          []string
	Count         int
	FlushCount    int
	FlushInterval int
	mu            sync.Mutex
	Sender        Sender
}

// Collector - query collector
type Collector struct {
	Tables        map[string]*Table
	mu            sync.Mutex
	Count         int
	FlushInterval int
	Sender        Sender
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
	return strings.Join(t.Rows, "\n")
}

// Flush - sends collcted data in table to clickhouse
func (t *Table) Flush() {
	rows := t.Content()
	resp, status := t.Sender.SendQuery(t.Name, rows)
	if status != http.StatusOK {
		log.Printf("Flush ERROR %+v: %+v\n", status, resp)
		Dump(t.Name, rows)
	}
	t.Rows = make([]string, 0, t.FlushCount)
	t.Count = 0
}

// CheckFluch - check if flush is need and sends data to clickhouse
func (t *Table) CheckFlush() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Count > 0 {
		t.Flush()
		return true
	}
	return false
}

func (t *Table) Empty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Count == 0
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
	count := strings.Count(text, "\n") + 1
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Count += count
	t.Rows = append(t.Rows, text)
	if len(t.Rows) >= t.FlushCount {
		t.Flush()
	}
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

// AddTable - adding table to collector
func (c *Collector) AddTable(name string) {
	t := NewTable(name, c.Sender, c.Count, c.FlushInterval)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Tables[name] = t
	t.RunTimer()
}

// Push - adding query to collector with query params (with query) and rows
func (c *Collector) Push(params string, content string) {
	_, ok := c.Tables[params]
	if !ok {
		//log.Printf("'%+v'\n", params)
		c.AddTable(params)
	}
	c.Tables[params].Add(content)
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
			q = queryString[i+6 : eoq]
			params = queryString[:i] + queryString[eoq:]
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
	return params, content, insert
}

// Parse - parsing text for query and data
func (c *Collector) Parse(text string) (prefix string, content string) {
	i := strings.Index(text, "FORMAT")
	if i >= 0 {
		w := false
		off := -1
		for c := i + 7; c < len(text); c++ {
			if !w && text[c] != ' ' && text[c] != '\n' {
				w = true
			}
			if w && (text[c] == ' ' || text[c] == '\n') {
				off = c + 1
				break
			}
		}
		if off >= 0 {
			prefix = text[:off]
			content = text[off:]
		}
	} else {
		i = strings.Index(text, "VALUES")
		if i >= 0 {
			prefix = text[:i+6]
			content = text[i+7:]
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
