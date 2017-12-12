package main

import "net/http"

// Sender interface for send requests
type Sender interface {
	Send(queryString string, data string)
	SendQuery(queryString string, data string) (response string, status int)
	Len() int64
	Empty() bool
	WaitFlush() (err error)
}

type fakeSender struct{}

func (s *fakeSender) Send(queryString string, data string) {}

func (s *fakeSender) SendQuery(queryString string, data string) (response string, status int) {
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
