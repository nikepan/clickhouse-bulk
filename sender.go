package main

import "net/http"

// Sender interface for send requests
type Sender interface {
	SendQuery(queryString string, data string) (response string, status int)
}

type fakeSender struct{}

func (s *fakeSender) SendQuery(queryString string, data string) (response string, status int) {
	return "", http.StatusOK
}
