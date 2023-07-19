package main

import (
	"bytes"
	"log"
	"net/http"
	"sync"
)

// Sender interface for send requests
type Sender interface {
	Send(r *ClickhouseRequest)
	SendQuery(r *ClickhouseRequest) (response string, status int, err error)
	PassThru(req *http.Request, clientReqBody []byte) (res *http.Response, buf *bytes.Buffer)
	Len() int64
	Empty() bool
	WaitFlush() (err error)
	SetCreds(c *Credentials)
}

type fakeSender struct {
	sendHistory      []string
	sendQueryHistory []string
	mu               sync.Mutex
}

func (s *fakeSender) Send(r *ClickhouseRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendHistory = append(s.sendHistory, r.Params+" "+r.Content)
}

func (s *fakeSender) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	s.sendQueryHistory = append(s.sendQueryHistory, r.Params+r.Content)
	log.Printf("DEBUG: send query: %+v\n", s.sendQueryHistory)
	return "", http.StatusOK, nil
}
func (c *fakeSender) PassThru(req *http.Request, clientReqBody []byte) (res *http.Response, buf *bytes.Buffer) {
	return
}

func (s *fakeSender) Len() int64 {
	return 0
}

func (s *fakeSender) Empty() bool {
	return true
}

func (s *fakeSender) WaitFlush() error {
	return nil
}

func (s *fakeSender) SetCreds(c *Credentials) {
}
