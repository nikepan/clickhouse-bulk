package main

// DualSender sends inserts to live ClickHouse first, then enqueues the same batch for backup.
// Each target has its own Clickhouse worker queue and FileDumper directory.
type DualSender struct {
	live   *Clickhouse
	backup *Clickhouse
}

// NewDualSender wires live (primary) and backup senders.
func NewDualSender(live, backup *Clickhouse) *DualSender {
	return &DualSender{live: live, backup: backup}
}

// Send delivers the batch to live, then to the backup queue (separate background worker).
func (d *DualSender) Send(r *ClickhouseRequest) {
	d.live.Send(r)
	dup := *r
	d.backup.Send(&dup)
}

// SendQuery is routed to live only (SELECT/DDL are not replicated to backup).
func (d *DualSender) SendQuery(r *ClickhouseRequest) (response string, status int, err error) {
	return d.live.SendQuery(r)
}

// Len returns combined queue depth of live and backup senders.
func (d *DualSender) Len() int64 {
	return d.live.Len() + d.backup.Len()
}

// Empty reports whether both sender queues are empty.
func (d *DualSender) Empty() bool {
	return d.live.Empty() && d.backup.Empty()
}

// WaitFlush waits until both senders finish their queues.
func (d *DualSender) WaitFlush() error {
	if err := d.live.WaitFlush(); err != nil {
		return err
	}
	return d.backup.WaitFlush()
}
