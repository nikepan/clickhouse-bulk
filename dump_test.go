package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type failSender struct {
	fakeSender
}

func (s *failSender) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	return "", http.StatusBadRequest, fmt.Errorf("wrong server status 400")
}

type downSender struct {
	fakeSender
}

func (s *downSender) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	return "", http.StatusBadGateway, ErrServerIsDown
}

func TestDump_Dump(t *testing.T) {
	c := NewClickhouse(-1, 10, 0, "", false)
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

	dumper.Listen(sender, 1)
	c.Dump("eee", "eee", "", "", 502)
	time.Sleep(time.Second * 2)
	err = dumper.ProcessNextDump(sender)
	assert.Equal(t, ErrNoDumps, err)

	err = os.Remove(dumpDir)
	assert.Nil(t, err)
}

func TestDump_ProcessNextDumpSkipsPoisonDump(t *testing.T) {
	dumpDir := "dumptest_poison"
	defer os.RemoveAll(dumpDir)
	dumper := NewDumper(dumpDir)
	assert.Nil(t, dumper.Dump("params1", "content1", "", "1", 400))
	assert.Nil(t, dumper.Dump("params2", "content2", "", "1", 400))

	sender := &failSender{}
	assert.Error(t, dumper.ProcessNextDump(sender))
	assert.Error(t, dumper.ProcessNextDump(sender))
	// both dumps failed permanently and must be skipped now,
	// not retried forever blocking the queue
	assert.True(t, errors.Is(dumper.ProcessNextDump(sender), ErrNoDumps))
}

func TestDump_ProcessNextDumpMalformedFile(t *testing.T) {
	dumpDir := "dumptest_malformed"
	defer os.RemoveAll(dumpDir)
	assert.Nil(t, os.Mkdir(dumpDir, 0o755))
	// a truncated dump: params line only, no data (e.g. disk was full)
	assert.Nil(t, os.WriteFile(path.Join(dumpDir, "dump1-1-0.dmp"), []byte("query=xxx"), 0o600))
	dumper := NewDumper(dumpDir)
	sender := &fakeSender{}
	assert.NotPanics(t, func() { dumper.ProcessNextDump(sender) })
	// corrupt dump must not block the queue
	assert.True(t, errors.Is(dumper.ProcessNextDump(sender), ErrNoDumps))
	assert.Empty(t, sender.sendQueryHistory)
}

func TestDump_FilePermissions(t *testing.T) {
	dumpDir := "dumptest_perms"
	defer os.RemoveAll(dumpDir)
	dumper := NewDumper(dumpDir)
	assert.Nil(t, dumper.Dump("params", "content", "", "1", 400))
	// dumps may contain credentials, keep them owner-only
	info, err := os.Stat(dumpDir)
	assert.Nil(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	fi, err := os.Stat(path.Join(dumpDir, dumper.dumpName(1, "1", 400)))
	assert.Nil(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
}

func TestDump_ProcessNextDumpRetriesWhenServerDown(t *testing.T) {
	dumpDir := "dumptest_down"
	defer os.RemoveAll(dumpDir)
	dumper := NewDumper(dumpDir)
	assert.Nil(t, dumper.Dump("params1", "content1", "", "1", 502))

	// transient failure must not lock the dump
	assert.Error(t, dumper.ProcessNextDump(&downSender{}))

	// once the server is back, the same dump must be sent and removed
	ok := &fakeSender{}
	assert.Nil(t, dumper.ProcessNextDump(ok))
	assert.Len(t, ok.sendQueryHistory, 1)
	assert.True(t, errors.Is(dumper.ProcessNextDump(ok), ErrNoDumps))
}
