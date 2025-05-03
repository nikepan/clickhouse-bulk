package main

import "sync"

type BulkFileDumper struct {
	mu         sync.Mutex
	clickhouse *Clickhouse
}

// Change Dump to match clickhouse.Dumper interface
func (fd *BulkFileDumper) Dump(params, content, response, prefix string, status int) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	// ...existing code for dumping files...
	return nil
}
