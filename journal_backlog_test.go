package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJournal_MaxPending(t *testing.T) {
	dir := "journaltest-limit"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	j, err := NewJournal(dir, false, 2)
	assert.Nil(t, err)

	_, err = j.Append("p", "a")
	assert.Nil(t, err)
	_, err = j.Append("p", "b")
	assert.Nil(t, err)
	_, err = j.Append("p", "c")
	assert.ErrorIs(t, err, ErrJournalBacklog)

	id1 := uint64(1)
	err = j.Ack([]uint64{id1})
	assert.Nil(t, err)

	_, err = j.Append("p", "d")
	assert.Nil(t, err)

	assert.Nil(t, j.Close())
}
