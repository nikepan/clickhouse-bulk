package main

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDump_Dump(t *testing.T) {
	c := NewClickhouse(-1, 10)
	dumpDir := "dumptest"
	dumper := NewDumper(dumpDir)
	c.Dumper = dumper
	c.AddServer("")
	c.Dump("eee", "eee")
	assert.True(t, c.Empty())
	buf, err := dumper.GetDumpData(dumper.dumpName(1))
	assert.Nil(t, err)
	assert.Equal(t, "eee\neee", string(buf))

	sender := &fakeSender{}
	err = dumper.ProcessNextDump(sender)
	assert.Nil(t, err)
	assert.Len(t, sender.sendQueryHistory, 1)
	err = dumper.ProcessNextDump(sender)
	assert.True(t, errors.Is(err, ErrNoDumps))
	assert.Len(t, sender.sendQueryHistory, 1)

	dumper.Listen(sender, 1)
	c.Dump("eee", "eee")
	time.Sleep(time.Second)
	err = dumper.ProcessNextDump(sender)
	assert.True(t, errors.Is(err, ErrNoDumps))

	err = os.Remove(dumpDir)
	assert.Nil(t, err)
}
