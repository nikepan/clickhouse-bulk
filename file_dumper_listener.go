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

// doSomeQueueWork simulates reading from a queue (returns nil unless forced to fail)
func doSomeQueueWork() error {
	if false { // e.g. a condition or test flag
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
