package main

import (
	"log"
	"net/http"
)

// Sender interface for send requests
type Sender interface {
	Send(queryString string, data string)
	SendQuery(queryString string, data string) (response string, status int)
	Len() int64
	Empty() bool
	WaitFlush() (err error)
}

type fakeSender struct {
	sendHistory      []string
	sendQueryHistory []string
}

func (s *fakeSender) Send(queryString string, data string) {
	s.sendHistory = append(s.sendHistory, queryString+" "+data)
}

func (s *fakeSender) SendQuery(queryString string, data string) (response string, status int) {
	s.sendQueryHistory = append(s.sendQueryHistory, queryString+" "+data)
	log.Printf("send query %+v\n", s.sendQueryHistory)
	return "", http.StatusOK
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
