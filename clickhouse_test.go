package main

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClickhouse_GetNextServer(t *testing.T) {
	c := NewClickhouse(300, 10, "", false)
	c.AddServer("", true)
	c.AddServer("http://127.0.0.1:8124", true)
	c.AddServer("http://127.0.0.1:8125", true)
	c.AddServer("http://127.0.0.1:8123", true)
	s := c.GetNextServer()
	assert.Equal(t, "", s.URL)
	s.SendQuery(&ClickhouseRequest{})
	s = c.GetNextServer()
	assert.Equal(t, "http://127.0.0.1:8124", s.URL)
	resp, status, err := s.SendQuery(&ClickhouseRequest{})
	assert.NotEqual(t, "", resp)
	assert.Equal(t, http.StatusBadGateway, status)
	assert.True(t, errors.Is(err, ErrServerIsDown))
	assert.Equal(t, true, s.Bad)
	c.SendQuery(&ClickhouseRequest{})
}

func TestClickhouse_Send(t *testing.T) {
	c := NewClickhouse(300, 10, "", false)
	c.AddServer("", true)
	c.Send(&ClickhouseRequest{})
	for !c.Queue.Empty() {
		time.Sleep(10)
	}
}

func TestClickhouse_SendQuery(t *testing.T) {
	c := NewClickhouse(300, 10, "", false)
	c.AddServer("", true)
	c.GetNextServer()
	c.Servers[0].Bad = true
	_, status, err := c.SendQuery(&ClickhouseRequest{})
	assert.Equal(t, 503, status)
	assert.True(t, errors.Is(err, ErrNoServers))
}

func TestClickhouse_SendQuery1(t *testing.T) {
	c := NewClickhouse(-1, 10, "", false)
	c.AddServer("", true)
	c.GetNextServer()
	c.Servers[0].Bad = true
	s := c.GetNextServer()
	assert.Equal(t, false, s.Bad)
}

func TestBulkFileDumper_Dump(t *testing.T) {
	// ...existing code...

	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}

	err := fd.Dump("param", "content", "response", "prefix", 200)
	if err != nil {
		t.Errorf("Dump returned an error: %v", err)
	}
}

func TestBulkFileDumper_Listener(t *testing.T) {
	// ...existing code...

	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}

	// In a real test, you'd run fd.Listen() in a goroutine
	// and possibly send data to be processed. Here we just
	// ensure the logic doesn't panic immediately.
	go fd.Listen()
}

func TestBulkFileDumper_ProcessNextDump(t *testing.T) {
	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}

	err := fd.ProcessNextDump()
	if err != nil {
		t.Errorf("ProcessNextDump returned an error: %v", err)
	}
}
