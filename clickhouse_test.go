package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClickhouse_Timeouts(t *testing.T) {
	c := NewClickhouse(300, 3, 45, "", false)
	c.AddServer("http://example.com", false)
	// send_timeout limits the whole request, not just connection setup:
	// a big batch insert must not be killed by the short connect timeout
	assert.Equal(t, 45*time.Second, c.Servers[0].Client.Timeout)

	def := NewClickhouse(300, 3, 0, "", false)
	def.AddServer("http://example.com", false)
	assert.Equal(t, 60*time.Second, def.Servers[0].Client.Timeout)
}

func TestClickhouse_GetNextServer(t *testing.T) {
	c := NewClickhouse(300, 10, 0, "", false)
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
	assert.Equal(t, true, s.IsBad())
	c.SendQuery(&ClickhouseRequest{})
}

func TestClickhouse_Send(t *testing.T) {
	c := NewClickhouse(300, 10, 0, "", false)
	c.AddServer("", true)
	c.Send(&ClickhouseRequest{})
	for !c.Queue.Empty() {
		time.Sleep(10)
	}
}

func TestClickhouse_SendQuery(t *testing.T) {
	c := NewClickhouse(300, 10, 0, "", false)
	c.AddServer("", true)
	c.GetNextServer()
	c.Servers[0].SetBad(true)
	_, status, err := c.SendQuery(&ClickhouseRequest{})
	assert.Equal(t, 503, status)
	assert.True(t, errors.Is(err, ErrNoServers))
}

func TestClickhouse_SendQuery1(t *testing.T) {
	c := NewClickhouse(60, 10, 0, "", false)
	c.AddServer("", true)
	c.GetNextServer()
	c.Servers[0].SetBad(true)
	// still inside down_timeout: server stays out of rotation
	assert.Nil(t, c.GetNextServer())
	// down_timeout elapsed: server is given another chance
	c.Servers[0].LastRequest = time.Now().Add(-61 * time.Second)
	s := c.GetNextServer()
	assert.NotNil(t, s)
	assert.Equal(t, false, s.IsBad())
}

// a down_timeout of 0 would unban a dead server instantly and turn the retry
// loop into an endless hot cycle, so it falls back to the default like the
// other timeouts do
func TestClickhouse_DownTimeoutDefaults(t *testing.T) {
	assert.Equal(t, 60, NewClickhouse(0, 10, 10, "", false).DownTimeout)
	assert.Equal(t, 60, NewClickhouse(-1, 10, 10, "", false).DownTimeout)
	assert.Equal(t, 300, NewClickhouse(300, 10, 10, "", false).DownTimeout)
}

// the retry loop must end after trying every server once, without relying on
// down_timeout to keep dead servers out of rotation
func TestClickhouse_SendQueryStopsAfterOneRoundOfServers(t *testing.T) {
	var hits int64
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer down.Close()

	// down_timeout deliberately shorter than a full cycle over the servers
	c := NewClickhouse(1, 10, 10, "", false)
	for i := 0; i < 4; i++ {
		c.AddServer(down.URL, false)
	}

	done := make(chan int, 1)
	go func() {
		_, status, _ := c.SendQuery(&ClickhouseRequest{Content: "test"})
		done <- status
	}()
	select {
	case status := <-done:
		assert.Equal(t, http.StatusServiceUnavailable, status)
		assert.Equal(t, int64(4), atomic.LoadInt64(&hits))
	case <-time.After(5 * time.Second):
		t.Fatalf("SendQuery did not give up, %+v requests sent to dead servers", atomic.LoadInt64(&hits))
	}
}

func TestClickhouse_ResponseBodyClosed(t *testing.T) {
	var closed bool
	body := &spyBody{onClose: func() { closed = true }}

	c := NewClickhouse(300, 10, 0, "", false)
	c.AddServer("http://example.com", false)
	srv := c.GetNextServer()
	srv.Client = &http.Client{
		Transport: &spyTransport{body: body},
	}

	srv.SendQuery(&ClickhouseRequest{Content: "test"})
	assert.True(t, closed)
}

func TestClickhouse_SendQueryDoesNotLogCredentials(t *testing.T) {
	c := NewClickhouse(300, 10, 0, "", false)
	c.AddServer("http://user:secretpass@127.0.0.1:8123", true)
	srv := c.GetNextServer()
	srv.Client = &http.Client{Transport: &spyTransport{body: &spyBody{onClose: func() {}}}}

	out := captureLog(func() {
		srv.SendQuery(&ClickhouseRequest{Query: "INSERT INTO t VALUES", Content: "(1)", Count: 1, isInsert: true})
	})

	assert.Contains(t, out, "127.0.0.1:8123")
	assert.NotContains(t, out, "secretpass")
}

type spyTransport struct{ body *spyBody }

func (t *spyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: t.body}, nil
}

type spyBody struct{ onClose func() }

func (b *spyBody) Read(p []byte) (int, error) { copy(p, "OK"); return 2, io.EOF }
func (b *spyBody) Close() error               { b.onClose(); return nil }

// server is marked down from request goroutines while the queue worker reads
// the flag for metrics, so the state has to be safe for concurrent access
func TestClickhouse_BadFlagIsRaceFree(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer s.Close()
	c := NewClickhouse(300, 10, 10, "", false)
	c.AddServer(s.URL, false)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.SendQuery(&ClickhouseRequest{Content: "test"})
		}()
		go func() {
			defer wg.Done()
			c.DumpServers()
		}()
	}
	wg.Wait()
	assert.Equal(t, true, c.Servers[0].IsBad())
}
