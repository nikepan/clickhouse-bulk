package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestRunServer(t *testing.T) {
	collector := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	server := InitServer("", collector, false)
	go server.Start()
	server.echo.POST("/", server.writeHandler)

	status, resp := request("POST", "/", "", server.echo)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = request("POST", "/?query="+escSelect, "", server.echo)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = request("POST", "/?query="+escTitle, qContent, server.echo)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = authRequest("POST", "default", "", "/?query="+escTitle, qContent, server.echo)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = authRequest("POST", "default", "", "/", "", server.echo)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	server.echo.GET("/status", server.statusHandler)
	status, _ = request("GET", "/status", "", server.echo)
	assert.Equal(t, status, http.StatusOK)

	server.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	status, _ = request("GET", "/metrics", "", server.echo)
	assert.Equal(t, status, http.StatusOK)

	server.echo.GET("/debug/gc", server.gcHandler)
	status, resp = request("GET", "/debug/gc", "", server.echo)
	assert.Equal(t, status, http.StatusOK)

	server.echo.GET("/debug/freemem", server.freeMemHandler)
	status, resp = request("GET", "/debug/freemem", "", server.echo)
	assert.Equal(t, status, http.StatusOK)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func TestServer_SafeQuit(t *testing.T) {
	sender := &fakeSender{}
	collect := NewCollector(sender, 1000, 1000, 0, true)
	collect.AddTable("test")
	collect.Push("sss", "sss")

	assert.False(t, collect.Empty())

	SafeQuit(collect, sender)

	assert.True(t, collect.Empty())
	assert.True(t, sender.Empty())
}

func TestServer_MultiServer(t *testing.T) {

	received := make([]string, 0)
	var mu sync.Mutex

	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "")
		req, _ := ioutil.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		received = append(received, string(req))
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "")
		req, _ := ioutil.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		received = append(received, string(req))
	}))
	defer s2.Close()

	sender := NewClickhouse(10, 10, "", false)
	sender.AddServer(s1.URL)
	sender.AddServer(s2.URL)
	collect := NewCollector(sender, 1000, 1000, 0, true)
	collect.AddTable("test")
	collect.Push("eee", "eee")
	collect.Push("fff", "fff")
	collect.Push("ggg", "ggg")

	assert.False(t, collect.Empty())

	SafeQuit(collect, sender)
	time.Sleep(100) // wait for http servers process requests

	assert.Equal(t, 3, len(received))
	assert.True(t, collect.Empty())
	assert.True(t, sender.Empty())

	os.Setenv("DUMP_CHECK_INTERVAL", "10")
	cnf, err := ReadConfig("wrong_config.json")
	os.Unsetenv("DUMP_CHECK_INTERVAL")
	assert.Nil(t, err)
	assert.Equal(t, 10, cnf.DumpCheckInterval)
	go RunServer(cnf)
	time.Sleep(1000)
}

func request(method, path string, body string, e *echo.Echo) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func authRequest(method, user string, password string, path string, body string, e *echo.Echo) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetBasicAuth(user, password)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}
