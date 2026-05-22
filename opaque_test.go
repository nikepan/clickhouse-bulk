package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldOpaqueInsert(t *testing.T) {
	q := url.QueryEscape("INSERT INTO t FORMAT Native")
	params := "query=" + q
	body := []byte{0x01, 0x02, 0x03}

	assert.True(t, shouldOpaqueInsert(false, "application/octet-stream", params, body))
	assert.True(t, shouldOpaqueInsert(false, "", params, body))
	assert.False(t, shouldOpaqueInsert(false, "", "query="+url.QueryEscape("INSERT INTO t FORMAT TabSeparated"), []byte("1\t2\n")))
	assert.True(t, shouldOpaqueInsert(true, "", "query="+url.QueryEscape("INSERT INTO t FORMAT TabSeparated"), []byte("1\t2\n")))
	assert.False(t, shouldOpaqueInsert(false, "application/octet-stream", "query="+url.QueryEscape("SELECT 1"), nil))
}

func TestOutboundContentType(t *testing.T) {
	assert.Equal(t, "application/octet-stream", outboundContentType("", "INSERT INTO t FORMAT Native"))
	assert.Equal(t, "text/custom", outboundContentType("text/custom; charset=utf-8", "INSERT INTO t FORMAT Native"))
}

func TestJournal_AppendOpaqueReplay(t *testing.T) {
	dir := "journalopaque"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	j, err := NewJournal(dir, false, 0)
	assert.Nil(t, err)

	payload := string([]byte{0xde, 0xad, 0xbe, 0xef})
	id, err := j.AppendOpaque("query=INSERT+FORMAT+Native", payload, "application/octet-stream")
	assert.Nil(t, err)

	sender := &fakeSender{}
	c := NewCollector(sender, j, 1000, 1000, 0, true, false)
	err = j.ReplayUnacked(c.ReplayJournalRecord)
	assert.Nil(t, err)

	sender.mu.Lock()
	n := len(sender.sendHistory)
	sender.mu.Unlock()
	assert.Equal(t, 1, n)
	assert.Contains(t, sender.sendHistory[0], "content_bytes=4")

	err = j.Ack([]uint64{id})
	assert.Nil(t, err)
}

func TestServer_OpaquePassthrough(t *testing.T) {
	var gotCT string
	var gotBody []byte
	var gotParams string

	ch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotParams = r.URL.RawQuery
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ch.Close()

	sender := NewClickhouse(10, 10, "", false, 0, 0)
	sender.AddServer(ch.URL, false)
	collector := NewCollector(sender, nil, 1000, 1000, 0, true, false)
	server := InitServer("", collector, sender, nil, nil, nil, false, false, false)
	server.echo.POST("/", server.writeHandler)

	nativeBody := "\x01\x02\x03\x04"
	q := url.QueryEscape("INSERT INTO events FORMAT Native")
	path := "/?query=" + q
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(nativeBody))
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "", rec.Body.String())

	SafeQuit(collector, sender, 5)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "application/octet-stream", gotCT)
	assert.Equal(t, nativeBody, string(gotBody))
	assert.Contains(t, gotParams, "query=")
}

func TestJournal_OpaqueRoundTripBase64(t *testing.T) {
	raw := []byte{0x00, 0xff, 0x42}
	b64 := base64.StdEncoding.EncodeToString(raw)
	dec, err := base64.StdEncoding.DecodeString(b64)
	assert.Nil(t, err)
	assert.Equal(t, raw, dec)
}
