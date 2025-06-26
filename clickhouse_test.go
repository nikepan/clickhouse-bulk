package main

import (
	"errors"
	"io"
	"net/http"
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

func TestClickhouse_ResponseBodyClosed(t *testing.T) {
	var closed bool
	body := &spyBody{onClose: func() { closed = true }}
	
	c := NewClickhouse(300, 10, "", false)
	c.AddServer("http://example.com", false)
	srv := c.GetNextServer()
	srv.Client = &http.Client{
		Transport: &spyTransport{body: body},
	}

	srv.SendQuery(&ClickhouseRequest{Content: "test"})
	assert.True(t, closed)
}

type spyTransport struct{ body *spyBody }
func (t *spyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: t.body}, nil
}

type spyBody struct{ onClose func() }
func (b *spyBody) Read(p []byte) (int, error) { copy(p, "OK"); return 2, io.EOF }
func (b *spyBody) Close() error { b.onClose(); return nil }
