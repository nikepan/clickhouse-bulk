package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestRunServer(t *testing.T) {
	cnf, _ := ReadConfig("wrong_config.json")
	collector := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	server := InitServer("", collector, false, true)
	go server.Start(cnf)
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

	status, resp = request("POST", "/?query="+escTitle, "", server.echo)
	assert.Equal(t, status, http.StatusInternalServerError)
	assert.Equal(t, resp, "Empty insert\n")

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
		req, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		received = append(received, string(req))
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "")
		req, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		received = append(received, string(req))
	}))
	defer s2.Close()

	sender := NewClickhouse(10, 10, 0, "", false)
	sender.AddServer(s1.URL, true)
	sender.AddServer(s2.URL, true)
	collect := NewCollector(sender, 1000, 1000, 0, true)
	collect.AddTable("test")
	collect.Push("eee", "eee")
	collect.Push("fff", "fff")
	collect.Push("ggg", "ggg")

	assert.False(t, collect.Empty())

	SafeQuit(collect, sender)
	time.Sleep(100 * time.Millisecond) // wait for http servers process requests

	assert.Equal(t, 3, len(received))
	assert.True(t, collect.Empty())
	assert.True(t, sender.Empty())
}

func TestServer_TablesCleanHandlerConcurrent(t *testing.T) {
	collector := NewCollector(&fakeSender{}, 1000, 1000, 0, true)
	server := InitServer("", collector, true, false)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			collector.Push(fmt.Sprintf("query=%d", i), "data")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			status, _ := request("GET", "/debug/tables-clean", "", server.echo)
			assert.Equal(t, http.StatusOK, status)
		}
	}()
	wg.Wait()
}

func TestServer_WriteHandlerStoresBeforeResponding(t *testing.T) {
	sender := &fakeSender{}
	collector := NewCollector(sender, 1, 1000, 0, true)
	server := InitServer("", collector, false, false)
	status, _ := request("POST", "/?query="+escTitle, qContent, server.echo)
	assert.Equal(t, http.StatusOK, status)
	sender.mu.Lock()
	defer sender.mu.Unlock()
	assert.Len(t, sender.sendHistory, 1)
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

// keep this test last in the package: RunServer leaks its goroutines,
// so nothing that touches shared globals may run after it
func TestZRunServerSmoke(t *testing.T) {
	os.Setenv("DUMP_CHECK_INTERVAL", "10")
	cnf, err := ReadConfig("wrong_config.json")
	os.Unsetenv("DUMP_CHECK_INTERVAL")
	assert.Nil(t, err)
	assert.Equal(t, 10, cnf.DumpCheckInterval)
	go RunServer(cnf)
	time.Sleep(time.Second)
}

func TestZRunServerGracefulShutdown(t *testing.T) {
	cnf, err := ReadConfig("wrong_config.json")
	assert.Nil(t, err)
	cnf.Listen = "127.0.0.1:18124"
	cnf.DumpCheckInterval = -1
	go RunServer(cnf)

	up := false
	for i := 0; i < 100; i++ {
		if _, err := http.Get("http://127.0.0.1:18124/status"); err == nil {
			up = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, up, "server did not start")

	// the shutdown context must not be created at startup: wait until a
	// startup-time 5s context would already be expired
	time.Sleep(6 * time.Second)
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := http.Get("http://127.0.0.1:18124/status"); err != nil {
			return // server stopped accepting, process still alive
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server still accepting connections after SIGTERM")
}
