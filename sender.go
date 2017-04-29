package main

import "net/http"

// Sender interface for send requests
type Sender interface {
	Send(queryString string, data string)
	SendQuery(queryString string, data string) (response string, status int)
}

type fakeSender struct{}

func (s *fakeSender) Send(queryString string, data string) {}

func (s *fakeSender) SendQuery(queryString string, data string) (response string, status int) {
	return "", http.StatusOK
}
