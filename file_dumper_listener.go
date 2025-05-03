package main

// ProcessNextDump processes the next item in the dumping queue.
func (fd *BulkFileDumper) ProcessNextDump() error {
	// Here you would handle the next dump task, e.g. reading data from a queue.
	return nil
}

func (fd *BulkFileDumper) Listen() {
	for {
		fd.mu.Lock()
		err := fd.ProcessNextDump()
		fd.mu.Unlock()

		if err != nil {
			continue
		}

		fd.clickhouse.mu.Lock()
		_, _, err = fd.clickhouse.SendQuery(&ClickhouseRequest{})
		fd.clickhouse.mu.Unlock()

		if err != nil {
		}
	}
}
