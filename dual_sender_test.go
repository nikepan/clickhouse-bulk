package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDualSender_Send(t *testing.T) {
	live := NewClickhouse(300, 10, "", false, 0, 0)
	live.AddServer("", true)
	backup := NewClickhouse(300, 10, "", false, 0, 0)
	backup.AddServer("", true)
	dual := NewDualSender(live, backup)

	dual.Send(&ClickhouseRequest{Params: "p", Content: "c", isInsert: true})

	deadline := time.Now().Add(2 * time.Second)
	for live.Len()+backup.Len() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, int64(0), dual.Len())
}
