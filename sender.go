package main

import (
	"log"
	"net/http"
	"sync"
)

// Sender interface for send requests
type Sender interface {
	Send(queryString string, data string)
	SendQuery(queryString string, data string) (response string, status int, err error)
	Len() int64
	Empty() bool
	WaitFlush() (err error)
}

type fakeSender struct {
	sendHistory      []string
	sendQueryHistory []string
	mu               sync.Mutex
}

func (s *fakeSender) Send(queryString string, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendHistory = append(s.sendHistory, queryString+" "+data)
}

func (s *fakeSender) SendQuery(queryString string, data string) (response string, status int, err error) {
	s.sendQueryHistory = append(s.sendQueryHistory, queryString+" "+data)
	log.Printf("DEBUG: send query: %+v\n", s.sendQueryHistory)
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
