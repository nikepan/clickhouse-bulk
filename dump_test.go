package main

import (
	"errors"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDump_Dump(t *testing.T) {
	c := NewClickhouse(-1, 10, "", false, 0, 0)
	dumpDir := "dumptest"
	dumper := NewDumper(dumpDir)
	c.Dumper = dumper
	c.AddServer("", true)
	c.Dump("eee", "eee", "error", "", 502)
	assert.True(t, c.Empty())
	buf, _, err := dumper.GetDumpData(dumper.dumpName(1, "", 502))
	assert.Nil(t, err)
	assert.Equal(t, "eee\neee", string(buf))

	sender := &fakeSender{}
	err = dumper.ProcessNextDump(sender)
	assert.Nil(t, err)
	assert.Len(t, sender.sendQueryHistory, 1)
	err = dumper.ProcessNextDump(sender)
	assert.True(t, errors.Is(err, ErrNoDumps))
	assert.Len(t, sender.sendQueryHistory, 1)

	dumper.Listen(sender, 1, 0)
	c.Dump("eee", "eee", "", "", 502)
	time.Sleep(time.Second * 2)
	err = dumper.ProcessNextDump(sender)
	assert.Equal(t, ErrNoDumps, err)

	err = os.RemoveAll(dumpDir)
	assert.Nil(t, err)
}

func TestDump_ClientErrorMovedToFailed(t *testing.T) {
	dumpDir := "dumptest-failed"
	dumper := NewDumper(dumpDir)
	dumper.DumpPrefix = "20990101120000"
	dumper.DumpNum = 0
	err := dumper.Dump("p=1", "data", "bad request", dumpKindClientError, 400)
	assert.Nil(t, err)
	name := dumper.dumpName(1, dumpKindClientError, 400)
	assert.True(t, isClientErrorDumpFile(name))

	sender := &fakeSender{}
	err = dumper.ProcessNextDump(sender)
	assert.Nil(t, err)
	assert.Len(t, sender.sendQueryHistory, 0)
	_, err = os.Stat(path.Join(dumpDir, failedDumpSubdir, name))
	assert.Nil(t, err)

	os.RemoveAll(dumpDir)
}

func TestDump_ReplayFailed(t *testing.T) {
	dumpDir := "dumptest-replay-failed"
	dumper := NewDumper(dumpDir)
	dumper.DumpPrefix = "20990101120000"
	dumper.DumpNum = 0
	err := dumper.Dump("p=1", "insert into t values (1)", "bad request", dumpKindClientError, 400)
	assert.Nil(t, err)
	name := dumper.dumpName(1, dumpKindClientError, 400)
	err = dumper.moveToFailed(name)
	assert.Nil(t, err)

	sender := &fakeSender{}
	report := dumper.ReplayFailed(sender, 0)
	assert.Equal(t, 1, report.Sent)
	assert.Equal(t, 0, report.Errors)
	assert.Equal(t, 0, report.Remaining)
	assert.Len(t, sender.sendQueryHistory, 1)

	os.RemoveAll(dumpDir)
}
