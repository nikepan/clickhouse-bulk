package main

import (
	"log"
	"net/http"
	"sync"
)

// Sender interface for send requests
type Sender interface {
	Send(r *ClickhouseRequest)
	SendQuery(r *ClickhouseRequest) (response string, status int, err error)
	Len() int64
	Empty() bool
	WaitFlush() (err error)
}

type fakeSender struct {
	sendHistory      []string
	sendQueryHistory []string
	mu               sync.Mutex
}

func (s *fakeSender) Send(r *ClickhouseRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendHistory = append(s.sendHistory, logInsertMeta(r.Params, r.Content))
}

func (s *fakeSender) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := logInsertMeta(r.Params, r.Content)
	s.sendQueryHistory = append(s.sendQueryHistory, meta)
	log.Printf("DEBUG: send query #%d %s\n", len(s.sendQueryHistory), meta)
	return "", http.StatusOK, nil
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
