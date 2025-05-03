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

func TestClickhouse_Send_QueueLen(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	qLen := c.Len()
	c.Send(&ClickhouseRequest{Params: "test"})
	if c.Len() != qLen+1 {
		t.Error("Expected queue length to increase after Send")
	}
}

// New test verifying that Run processes at least one request
func TestClickhouse_Run_OneRequest(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	c.AddServer("", false) // Force an empty URL server
	go c.Run()
	c.Send(&ClickhouseRequest{Params: "test"})
	time.Sleep(500 * time.Millisecond)
	if !c.Empty() {
		t.Error("Expected queue to be emptied after processing one request")
	}
}

// New test to confirm SendQuery returns no error with empty URL
func TestSendQuery_EmptyURL(t *testing.T) {
	srv := &ClickhouseServer{}
	resp, status, err := srv.SendQuery(&ClickhouseRequest{})
	if resp != "" || status != http.StatusOK || err != nil {
		t.Errorf("Expected no error with empty URL, got status=%d err=%v resp=%q",
			status, err, resp)
	}
}

// New test for a valid prefix in BulkFileDumper
func TestBulkFileDumper_Dump_ValidPrefix(t *testing.T) {
	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}
	err := fd.Dump("param", "content", "response", "valid", 200)
	if err != nil {
		t.Errorf("Dump returned an unexpected error: %v", err)
	}
}

// New test verifying no error in ProcessNextDump success scenario
func TestBulkFileDumper_ProcessNextDump_Success(t *testing.T) {
	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}
	err := fd.ProcessNextDump()
	if err != nil {
		t.Errorf("Did not expect an error, got: %v", err)
	}
}
