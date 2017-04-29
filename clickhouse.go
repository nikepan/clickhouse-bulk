package main

import (
	"github.com/nikepan/go-datastructures/queue"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ClickhouseServer - clickhouse server instance object struct
type ClickhouseServer struct {
	URL         string
	LastRequest time.Time
	Bad         bool
	Client      *http.Client
}

// Clickhouse - main clickhouse sender object
type Clickhouse struct {
	Servers     []*ClickhouseServer
	Queue       *queue.Queue
	mu          sync.Mutex
	DownTimeout int
}

// ClickhouseRequest - request struct for queue
type ClickhouseRequest struct {
	Params  string
	Content string
}

// NewClickhouse - get clickhouse object
func NewClickhouse(downTimeout int) (c *Clickhouse) {
	c = new(Clickhouse)
	c.DownTimeout = downTimeout
	c.Servers = make([]*ClickhouseServer, 0)
	c.Queue = queue.New(1000)
	go c.Run()
	return c
}

// AddServer - add clickhouse server url
func (c *Clickhouse) AddServer(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Servers = append(c.Servers, &ClickhouseServer{URL: url, Client: &http.Client{}})
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
func (c *Clickhouse) Send(queryString string, data string) {
	req := ClickhouseRequest{queryString, data}
	c.Queue.Put(req)
}

// Dump - save query to file
func (c *Clickhouse) Dump(params string, data string) {

}

// Run server
func (c *Clickhouse) Run() {
	var err error
	var datas []interface{}
	for {
		datas, err = c.Queue.Poll(1, time.Second*5)
		if err == nil {
			data := datas[0].(ClickhouseRequest)
			resp, status := c.SendQuery(data.Params, data.Content)
			if status != http.StatusOK {
				log.Printf("Send ERROR %+v: %+v\n", status, resp)
				c.Dump(data.Params, data.Content)
			}
		}
	}
}

// SendQuery - sends query to server and return result
func (srv *ClickhouseServer) SendQuery(queryString string, data string) (response string, status int) {
	if srv.URL != "" {

		log.Printf("send %+v rows to %+v of %+v\n", strings.Count(data, "\n")+1, srv.URL, queryString)

		resp, err := srv.Client.Post(srv.URL+"?"+queryString, "", strings.NewReader(data))
		if err != nil {
			srv.Bad = true
			return err.Error(), http.StatusBadGateway
		}
		buf, _ := ioutil.ReadAll(resp.Body)
		s := string(buf)
		return s, resp.StatusCode
	}

	return "", http.StatusOK
}

// SendQuery - sends query to server and return result (with server cycle)
func (c *Clickhouse) SendQuery(queryString string, data string) (response string, status int) {
	for {
		s := c.GetNextServer()
		if s != nil {
			r, status := s.SendQuery(queryString, data)
			if status == http.StatusBadGateway {
				continue
			}
			return r, status
		} else {
			return "No working clickhouse servers", http.StatusBadGateway
		}
	}
}
