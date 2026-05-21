package main

// ServerStatus is a single ClickHouse endpoint health snapshot.
type ServerStatus struct {
	URL string `json:"url"`
	Bad bool `json:"bad"`
}

// TargetStatus describes one sender target (live or backup).
type TargetStatus struct {
	Enabled   bool           `json:"enabled"`
	QueueLen  int64          `json:"queue_len"`
	Servers   []ServerStatus `json:"servers"`
}

// FullStatus is returned by GET /status.
type FullStatus struct {
	Status string       `json:"status"`
	Live   TargetStatus `json:"live"`
	Backup TargetStatus `json:"backup"`
}

func buildTargetStatus(ch *Clickhouse, enabled bool) TargetStatus {
	if ch == nil || !enabled {
		return TargetStatus{Enabled: false}
	}
	return TargetStatus{
		Enabled:  true,
		QueueLen: ch.Len(),
		Servers:  ch.ServersSnapshot(),
	}
}
