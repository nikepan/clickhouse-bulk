package main

import (
	"fmt"
	"log"
	"sync"
)

type BulkFileDumper struct {
	mu         sync.Mutex
	clickhouse *Clickhouse
}

// Dump implements the Dumper interface
func (fd *BulkFileDumper) Dump(params, content, response, prefix string, status int) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	err := doSomeDumpLogic(params, content, response, prefix, status)
	if err != nil {
		log.Printf("Dump error: %v", err)
		return err
	}
	return nil
}

func doSomeDumpLogic(params, content, response, prefix string, status int) error {
	if prefix == "" {
		return fmt.Errorf("prefix cannot be empty")
	}
	return nil
}
