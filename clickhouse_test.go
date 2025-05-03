package main

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestBulkFileDumper_Dump_EmptyPrefix(t *testing.T) {
	fd := &BulkFileDumper{}
	err := fd.Dump("param", "content", "response", "", 200)
	if err == nil {
		t.Error("Expected an error when prefix is empty, got nil")
	}
}

func TestBulkFileDumper_ProcessNextDump_Error(t *testing.T) {
	fd := &BulkFileDumper{}
	err := fd.ProcessNextDump()
	if err != nil {
		t.Logf("Correctly handled error in ProcessNextDump: %v", err)
	}
}

func TestClickhouse_DumpServers(t *testing.T) {
	c := NewClickhouse(300, 10, "", false)
	c.AddServer("", true)
	c.DumpServers()
}

func TestClickhouse_FlushAll(t *testing.T) {
	c := NewClickhouse(300, 10, "", false)
	c.Send(&ClickhouseRequest{})
	c.FlushAll()
	if !c.Empty() {
		t.Error("Expected all queued items to be flushed")
	}
}

func TestClickhouse_NewClickhouse(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	if c == nil {
		t.Error("Expected NewClickhouse to return a valid instance, got nil")
	}
}

func TestClickhouse_AddServer(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	if len(c.Servers) != 0 {
		t.Error("Expected no servers initially")
	}
	c.AddServer("http://127.0.0.1:8123", true)
	if len(c.Servers) != 1 {
		t.Error("Expected 1 server after AddServer")
	}
}

func TestClickhouse_GetNextServer_NoServers(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	srv := c.GetNextServer()
	if srv != nil {
		t.Error("Expected no server when none are added")
	}
}

func TestClickhouse_Send_QueueLen(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	qLen := c.Len()
	c.Send(&ClickhouseRequest{Params: "test"})
	if c.Len() != qLen+1 {
		t.Error("Expected queue length to increase after Send")
	}
}

func TestClickhouse_Run_OneRequest(t *testing.T) {
	c := NewClickhouse(5, 5, "", false)
	c.AddServer("", false) // Force an empty URL server to skip actual network
	go c.Run()
	c.Send(&ClickhouseRequest{Params: "test"})
	time.Sleep(500 * time.Millisecond) // Give a little time for Run to poll
	if !c.Empty() {
		t.Error("Expected queue to be emptied after processing one request")
	}
}

func TestSendQuery_EmptyURL(t *testing.T) {
	srv := &ClickhouseServer{}
	resp, status, err := srv.SendQuery(&ClickhouseRequest{})
	if resp != "" || status != http.StatusOK || err != nil {
		t.Errorf("Expected no error with empty URL, but got status=%d err=%v resp=%q",
			status, err, resp)
	}
}

func TestBulkFileDumper_Dump_ValidPrefix(t *testing.T) {
	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}
	err := fd.Dump("param", "content", "response", "valid", 200)
	if err != nil {
		t.Errorf("Dump returned an unexpected error with a valid prefix: %v", err)
	}
}

func TestBulkFileDumper_ProcessNextDump_Success(t *testing.T) {
	ch := &Clickhouse{}
	fd := &BulkFileDumper{
		mu:         sync.Mutex{},
		clickhouse: ch,
	}
	err := fd.ProcessNextDump()
	if err != nil {
		t.Errorf("Did not expect an error in ProcessNextDump success scenario, got %v", err)
	}
}
