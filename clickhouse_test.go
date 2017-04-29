package main

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

func TestClickhouse_GetNextServer(t *testing.T) {
	c := NewClickhouse(300)
	c.AddServer("")
	c.AddServer("http://127.0.0.1:8124")
	c.AddServer("http://127.0.0.1:8125")
	c.AddServer("http://127.0.0.1:8123")
	s := c.GetNextServer()
	assert.Equal(t, s.URL, "")
	s.SendQuery("", "")
	s = c.GetNextServer()
	assert.Equal(t, s.URL, "http://127.0.0.1:8124")
	resp, status := s.SendQuery("", "")
	assert.NotEqual(t, resp, "")
	assert.Equal(t, status, http.StatusBadGateway)
	assert.Equal(t, s.Bad, true)
	c.SendQuery("", "")
}

func TestClickhouse_Send(t *testing.T) {
	c := NewClickhouse(300)
	c.AddServer("")
	c.Send("", "")
	for !c.Queue.Empty() {
		time.Sleep(10)
	}
}

func TestClickhouse_SendQuery(t *testing.T) {
	c := NewClickhouse(300)
	c.AddServer("")
	c.GetNextServer()
	c.Servers[0].Bad = true
	_, status := c.SendQuery("", "")
	assert.Equal(t, status, http.StatusBadGateway)
}

func TestClickhouse_SendQuery1(t *testing.T) {
	c := NewClickhouse(-1)
	c.AddServer("")
	c.GetNextServer()
	c.Servers[0].Bad = true
	s := c.GetNextServer()
	assert.Equal(t, s.Bad, false)
}
