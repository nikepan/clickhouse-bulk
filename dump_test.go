package main

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDump_Dump(t *testing.T) {
	const dumpName = "dump1.dmp"
	c := NewClickhouse(-1)
	dumper := new(FileDumper)
	dumpDir := "dumptest"
	dumper.Path = dumpDir
	c.Dumper = dumper
	c.AddServer("")
	c.Dump("eee", "eee")
	assert.True(t, c.Empty())
	buf, err := dumper.GetDumpData(dumpName)
	assert.Nil(t, err)
	assert.Equal(t, "eee\neee", string(buf))

	sender := &fakeSender{}
	err = dumper.ProcessNextDump(sender)
	assert.Nil(t, err)
	assert.Len(t, sender.sendQueryHistory, 1)
	err = dumper.ProcessNextDump(sender)
	assert.True(t, errors.Is(err, ErrNoDumps))
	assert.Len(t, sender.sendQueryHistory, 1)
	os.Remove(dumpDir)
}
