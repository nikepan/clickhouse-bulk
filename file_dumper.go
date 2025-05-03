package main

import "sync"

type BulkFileDumper struct {
	mu         sync.Mutex
	clickhouse *Clickhouse
}

// Dump implements the Dumper interface
func (fd *BulkFileDumper) Dump(params, content, response, prefix string, status int) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	return nil
}
