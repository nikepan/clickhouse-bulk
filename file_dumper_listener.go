package main

func (fd *BulkFileDumper) ProcessNextDump() error {
	// ...existing code...
	return nil
}

func (fd *BulkFileDumper) Listen() {
	for {
		fd.mu.Lock()
		err := fd.ProcessNextDump()
		fd.mu.Unlock()

		if err != nil {
			// Handle error
			continue
		}

		fd.clickhouse.mu.Lock()
		_, _, err = fd.clickhouse.SendQuery(&ClickhouseRequest{})
		fd.clickhouse.mu.Unlock()

		if err != nil {
			// Handle error
		}
	}
}
