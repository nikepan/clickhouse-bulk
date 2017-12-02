package main

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestClickhouse_GetNextServer(t *testing.T) {
	c := NewClickhouse(300)
	c.AddServer("")
	c.AddServer("http://127.0.0.1:8124")
	c.AddServer("http://127.0.0.1:8125")
	c.AddServer("http://127.0.0.1:8123")
	s := c.GetNextServer()
	assert.Equal(t, "", s.URL)
	s.SendQuery("", "")
	s = c.GetNextServer()
	assert.Equal(t, "http://127.0.0.1:8124", s.URL)
	resp, status := s.SendQuery("", "")
	assert.NotEqual(t, "", resp)
	assert.Equal(t, http.StatusBadGateway, status)
	assert.Equal(t, true, s.Bad)
	c.SendQuery("", "")
}

func TestClickhouse_SendQuery(t *testing.T) {
	c := NewClickhouse(300)
	c.AddServer("")
	c.GetNextServer()
	c.Servers[0].Bad = true
	_, status := c.SendQuery("", "")
	assert.Equal(t, http.StatusBadGateway, status)
}

func TestClickhouse_SendQuery1(t *testing.T) {
	c := NewClickhouse(-1)
	c.AddServer("")
	c.GetNextServer()
	c.Servers[0].Bad = true
	s := c.GetNextServer()
	assert.Equal(t, false, s.Bad)
}
