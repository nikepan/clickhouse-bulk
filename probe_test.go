package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeDownTimeoutZero(t *testing.T) {
	var hits int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer s.Close()

	c := NewClickhouse(0, 10, 10, "", false)
	c.AddServer(s.URL, false)

	done := make(chan int)
	go func() {
		_, status, _ := c.SendQuery(&ClickhouseRequest{Content: "x"})
		done <- status
	}()

	select {
	case st := <-done:
		t.Logf("RETURNED status=%d after %d hits", st, atomic.LoadInt64(&hits))
	case <-time.After(2 * time.Second):
		t.Errorf("HUNG: SendQuery did not return in 2s, %d requests hammered at dead server", atomic.LoadInt64(&hits))
	}
}

func TestProbeDownTimeoutPositive(t *testing.T) {
	var hits int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer s.Close()

	c := NewClickhouse(60, 10, 10, "", false)
	c.AddServer(s.URL, false)

	done := make(chan int)
	go func() {
		_, status, _ := c.SendQuery(&ClickhouseRequest{Content: "x"})
		done <- status
	}()
	select {
	case st := <-done:
		t.Logf("RETURNED status=%d after %d hits", st, atomic.LoadInt64(&hits))
	case <-time.After(2 * time.Second):
		t.Errorf("HUNG with down_timeout=60")
	}
}

// down_timeout > 0, but shorter than the time to cycle through all servers
func TestProbeShortDownTimeoutManyServers(t *testing.T) {
	var hits int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer s.Close()

	c := NewClickhouse(1, 10, 10, "", false)
	for i := 0; i < 8; i++ {
		c.AddServer(s.URL, false)
	}

	done := make(chan int)
	go func() {
		_, status, _ := c.SendQuery(&ClickhouseRequest{Content: "x"})
		done <- status
	}()
	select {
	case st := <-done:
		t.Logf("RETURNED status=%d after %d hits", st, atomic.LoadInt64(&hits))
	case <-time.After(5 * time.Second):
		t.Errorf("HUNG: down_timeout=1 with 8 slow servers, %d hits", atomic.LoadInt64(&hits))
	}
}
