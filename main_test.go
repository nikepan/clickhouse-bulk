package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMain_MultiServer(t *testing.T) {

	servers := make(map[string]string)
	var mu sync.Mutex

	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "")
		req, _ := ioutil.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		servers["s1"] = string(req)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "")
		req, _ := ioutil.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		servers["s2"] = string(req)
	}))
	defer s2.Close()

	sender := NewClickhouse(10)
	sender.AddServer(s1.URL)
	sender.AddServer(s2.URL)
	collect := NewCollector(sender, 1000, 1000)
	collect.AddTable("test")
	collect.Push("eee", "eee")
	collect.Push("fff", "fff")
	collect.Push("ggg", "ggg")

	assert.False(t, collect.Empty())

	SafeQuit(collect, sender)
	time.Sleep(100) // wait for http servers process requests

	if servers["s1"] == "ggg" {
		assert.Equal(t, "fff", servers["s2"])
	} else if servers["s1"] == "fff" {
		assert.Equal(t, "ggg", servers["s2"])
	}

	assert.True(t, collect.Empty())
	assert.True(t, sender.Empty())
}

func TestMain_SafeQuit(t *testing.T) {
	sender := &fakeSender{}
	collect := NewCollector(sender, 1000, 1000)
	collect.AddTable("test")
	collect.Push("sss", "sss")

	assert.False(t, collect.Empty())

	SafeQuit(collect, sender)

	assert.True(t, collect.Empty())
	assert.True(t, sender.Empty())
}
