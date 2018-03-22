package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMain_SafeQuit(t *testing.T) {
	sender := &fakeSender{}
	collect := NewCollector(sender, 1000, 1000)
	collect.AddTable("test")
	collect.Push("eee", "eee")

	assert.False(t, collect.Empty())

	SafeQuit(collect, sender)

	assert.True(t, collect.Empty())
	assert.True(t, sender.Empty())
}
