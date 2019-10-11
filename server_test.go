package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
)

func TestRunServer(t *testing.T) {
	collector := NewCollector(&fakeSender{}, 1000, 1000)
	server := NewServer("", collector, false)
	e := echo.New()
	e.POST("/", server.writeHandler)

	status, resp := request("POST", "/", "", e)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = request("POST", "/?query="+escSelect, "", e)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = request("POST", "/?query="+escTitle, qContent, e)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = authRequest("POST", "default", "", "/?query="+escTitle, qContent, e)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	status, resp = authRequest("POST", "default", "", "/", "", e)
	assert.Equal(t, status, http.StatusOK)
	assert.Equal(t, resp, "")

	e.GET("/status", server.statusHandler)
	status, _ = request("GET", "/status", "", e)
	assert.Equal(t, status, http.StatusOK)
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
