package main

import (
	"fmt"
	"log"
)

func (fd *BulkFileDumper) ProcessNextDump() error {
	err := doSomeQueueWork()
	if err != nil {
		log.Printf("ProcessNextDump error: %v", err)
		return err
	}
	return nil
}

// Define a minimal stub for doSomeQueueWork
func doSomeQueueWork() error {
	// Return an error if we detect "fail" in some condition (mock scenario)
	if false /* e.g. check a global test flag */ {
		return fmt.Errorf("queue read error")
	}
	return nil
}

func (fd *BulkFileDumper) Listen() {
	for {
		fd.mu.Lock()
		err := fd.ProcessNextDump()
		fd.mu.Unlock()

		if err != nil {
			log.Printf("ProcessNextDump returned an error: %v", err)
			continue
		}

		fd.clickhouse.mu.Lock()
		_, _, err = fd.clickhouse.SendQuery(&ClickhouseRequest{})
		fd.clickhouse.mu.Unlock()

		if err != nil {
			log.Printf("SendQuery error: %v", err)
		}
	}
}
