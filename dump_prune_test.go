package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileDumper_pruneOldest(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "bulk-prune-test")
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	d := NewDumper(dir)
	d.DumpPrefix = "20990101120000"
	d.MaxDumpFiles = 2

	for i := 0; i < 3; i++ {
		err := d.Dump("p", "d", "", dumpKindTransient, 502)
		assert.Nil(t, err)
	}

	files, err := d.listPendingDumpFiles()
	assert.Nil(t, err)
	assert.Len(t, files, 2)
	assert.Equal(t, d.dumpName(2, dumpKindTransient, 502), files[0])
	assert.Equal(t, d.dumpName(3, dumpKindTransient, 502), files[1])
}
