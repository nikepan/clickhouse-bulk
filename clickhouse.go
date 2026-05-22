package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nikepan/go-datastructures/queue"
)

// ClickhouseServer - clickhouse server instance object struct
type ClickhouseServer struct {
	URL         string
	LastRequest time.Time
	Bad         bool
	Client      *http.Client
	LogQueries  bool
}

// Clickhouse - main clickhouse sender object
type Clickhouse struct {
	Servers        []*ClickhouseServer
	Queue          *queue.Queue
	mu             sync.Mutex
	DownTimeout    int
	ConnectTimeout int
	Dumper         Dumper
	QueryParams    string // appended to every request (&-separated)
	MetricTarget   string // "" or "live" for primary; "backup" for clickhouse-backup
	Journal        *Journal
	sendLimiter    *sendRateLimiter
	wg             sync.WaitGroup
	Transport      *http.Transport
}

// ClickhouseRequest - request struct for queue
type ClickhouseRequest struct {
	Params      string
	Query       string
	Content     string
	ContentType string // outbound POST type; empty = text/plain
	Count       int
	JournalIDs  []uint64
	isInsert    bool
	opaque      bool // true: body is verbatim client payload (no batch merge)
}

// ErrServerIsDown - signals about server is down
var ErrServerIsDown = errors.New("server is down")

// ErrNoServers - signals about no working servers
var ErrNoServers = errors.New("No working clickhouse servers")

// NewClickhouse - get clickhouse object
func NewClickhouse(downTimeout int, connectTimeout int, tlsServerName string, tlsSkipVerify bool, sendMaxRPS, sendMaxBurst int) (c *Clickhouse) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if tlsServerName != "" {
		tlsConfig.ServerName = tlsServerName
	}
	if tlsSkipVerify == true {
		tlsConfig.InsecureSkipVerify = tlsSkipVerify
	}

	c = new(Clickhouse)
	c.DownTimeout = downTimeout
	c.ConnectTimeout = connectTimeout
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 10
	}
	c.Servers = make([]*ClickhouseServer, 0)
	c.Queue = queue.New(1000)
	c.sendLimiter = newSendRateLimiter(sendMaxRPS, sendMaxBurst)
	c.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	go c.Run()
	return c
}

// AddServer - add clickhouse server url
func (c *Clickhouse) AddServer(url string, logQueries bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Servers = append(c.Servers, &ClickhouseServer{URL: url, Client: &http.Client{
		Timeout: time.Second * time.Duration(c.ConnectTimeout), Transport: c.Transport,
	}, LogQueries: logQueries })
}

// DumpServers - dump servers state to prometheus
func (c *Clickhouse) DumpServers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	good := 0
	bad := 0
	for _, s := range c.Servers {
		if s.Bad {
			bad++
		} else {
			good++
		}
	}
	setServerGauges(c.MetricTarget, good, bad)
	setSendQueueGauge(c.MetricTarget, c.Queue.Len())
}

// GetNextServer - getting next server for request
func (c *Clickhouse) GetNextServer() (srv *ClickhouseServer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tnow := time.Now()
	for _, s := range c.Servers {
		if s.Bad {
			if tnow.Sub(s.LastRequest) > time.Second*time.Duration(c.DownTimeout) {
				s.Bad = false
			} else {
				continue
			}
		}
		if srv != nil {
			if srv.LastRequest.Sub(s.LastRequest) > 0 {
				srv = s
			}
		} else {
			srv = s
		}
	}
	if srv != nil {
		srv.LastRequest = time.Now()
	}
	return srv

}

// Send - send request to next server
func (c *Clickhouse) Send(r *ClickhouseRequest) {
	c.wg.Add(1)
	c.Queue.Put(r)
}

// ackJournal clears WAL entries once data is durably stored (ClickHouse or dump_dir).
func (c *Clickhouse) ackJournal(ids []uint64) {
	if c.Journal == nil || len(ids) == 0 {
		return
	}
	if err := c.Journal.Ack(ids); err != nil {
		log.Printf("ERROR: journal ack: %+v\n", err)
		return
	}
	setJournalPendingGauge(c.Journal)
}

// Dump - save query to file
func (c *Clickhouse) Dump(params string, content string, response string, prefix string, status int) error {
	incDumpCounter(c.MetricTarget)
	if c.Dumper != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.Dumper.Dump(params, content, response, prefix, status)
	}
	return nil
}

// Len - returns queries queue length
func (c *Clickhouse) Len() int64 {
	return c.Queue.Len()
}

// Empty - check if queue is empty
func (c *Clickhouse) Empty() bool {
	return c.Queue.Empty()
}

// Run server
func (c *Clickhouse) Run() {
	var err error
	var datas []interface{}
	for {
		datas, err = c.Queue.Poll(1, time.Second*5)
		if err == nil {
			data := datas[0].(*ClickhouseRequest)
			resp, status, err := c.SendQuery(data)
			if err != nil {
				log.Printf("ERROR: Send status=%+v err=%+v response=%s\n", status, err, logTruncate(resp, 256))
				prefix := "1"
				if status >= 400 && status < 502 {
					prefix = "2"
				}
				if dumpErr := c.Dump(data.Params, data.Content, resp, prefix, status); dumpErr != nil {
					log.Printf("ERROR: dump failed, journal entries retained: %+v\n", dumpErr)
				} else {
					c.ackJournal(data.JournalIDs)
				}
			} else {
				incSentCounter(c.MetricTarget)
				recordLastSent(c.MetricTarget)
				c.ackJournal(data.JournalIDs)
			}
			c.DumpServers()
			c.wg.Done()
		}
	}
}

// WaitFlush - wait for flush ends
func (c *Clickhouse) WaitFlush() (err error) {
	c.wg.Wait()
	return nil
}

// SendQuery - sends query to server and return result
func (srv *ClickhouseServer) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	if srv.URL != "" {
		url := srv.URL
		if r.Params != "" {
			url += "?" + r.Params
		}
		if r.isInsert && srv.LogQueries {
			log.Printf("INFO: sending %+v rows to %+v query=%q\n", r.Count, srv.URL, logTruncate(r.Query, 200))
		}
		ct := r.ContentType
		if ct == "" {
			ct = "text/plain"
		}
		resp, err := srv.Client.Post(url, ct, strings.NewReader(r.Content))
		if err != nil {
			srv.Bad = true
			return err.Error(), http.StatusBadGateway, ErrServerIsDown
		}
		defer resp.Body.Close()
		if r.isInsert && srv.LogQueries {
			log.Printf("INFO: sent %+v rows to %+v query=%q\n", r.Count, srv.URL, logTruncate(r.Query, 200))
		}
		buf, _ := io.ReadAll(resp.Body)
		s := string(buf)
		if resp.StatusCode >= 502 {
			srv.Bad = true
			err = ErrServerIsDown
		} else if resp.StatusCode >= 400 {
			err = fmt.Errorf("wrong server status %d: response: %s (request %d bytes)",
				resp.StatusCode, logTruncate(s, 512), len(r.Content))
		}
		return s, resp.StatusCode, err
	}

	return "", http.StatusOK, err
}

// ServersSnapshot returns URL and Bad flag for each configured server.
func (c *Clickhouse) ServersSnapshot() []ServerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ServerStatus, 0, len(c.Servers))
	for _, s := range c.Servers {
		out = append(out, ServerStatus{URL: s.URL, Bad: s.Bad})
	}
	return out
}

// SendQuery - sends query to server and return result (with server cycle)
func (c *Clickhouse) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	if c.sendLimiter != nil {
		c.sendLimiter.Wait()
	}
	req := *r
	if c.QueryParams != "" {
		req.Params = mergeQueryParams(r.Params, c.QueryParams)
	}
	for {
		s := c.GetNextServer()
		if s != nil {
			response, status, err = s.SendQuery(&req)
			if errors.Is(err, ErrServerIsDown) {
				log.Printf("ERROR: server down (%+v): %+v\n", status, response)
				continue
			}
			return response, status, err
		}
		return "", http.StatusServiceUnavailable, ErrNoServers
	}
}
