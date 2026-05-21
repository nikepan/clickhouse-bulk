package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const metricTargetBackup = "backup"

var pushCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_received_count",
		Help: "Received requests count from launch",
	})
var sentCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_sent_count",
		Help: "Sent batches to live ClickHouse from launch",
	})
var dumpCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_dump_count",
		Help: "Dumps saved for live ClickHouse from launch",
	})
var goodServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_good_servers",
		Help: "Live ClickHouse servers in good state",
	})
var badServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bad_servers",
		Help: "Live ClickHouse servers marked bad",
	})
var queuedDumps = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_queued_dumps",
		Help: "Dump files waiting in live dump_dir",
	})
var sendQueue = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_send_queue",
		Help: "Live sender queue length",
	})

var sentBkpCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_bkp_sent_count",
		Help: "Sent batches to backup ClickHouse from launch",
	})
var dumpBkpCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_bkp_dump_count",
		Help: "Dumps saved for backup ClickHouse from launch",
	})
var goodBkpServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bkp_good_servers",
		Help: "Backup ClickHouse servers in good state",
	})
var badBkpServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bkp_bad_servers",
		Help: "Backup ClickHouse servers marked bad",
	})
var queuedBkpDumps = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bkp_queued_dumps",
		Help: "Dump files waiting in backup dump_dir",
	})
var sendBkpQueue = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bkp_send_queue",
		Help: "Backup sender queue length",
	})

var lastSentUnix = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_last_sent_unixtime",
		Help: "Unix time of last successful live batch send",
	})
var lastBkpSentUnix = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bkp_last_sent_unixtime",
		Help: "Unix time of last successful backup batch send",
	})

var dumpDirBytes = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_dump_dir_bytes",
		Help: "Total size in bytes of live dump_dir (including failed/)",
	})
var dumpBkpDirBytes = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bkp_dump_dir_bytes",
		Help: "Total size in bytes of backup dump_dir (including failed/)",
	})

var backupMetricsEnabled bool

var journalPending = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_journal_pending",
		Help: "Journal WAL records not yet durably stored (live CH or dump_dir)",
	})

func isBackupTarget(target string) bool {
	return target == metricTargetBackup
}

func incSentCounter(target string) {
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			sentBkpCounter.Inc()
		}
		return
	}
	sentCounter.Inc()
}

func incDumpCounter(target string) {
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			dumpBkpCounter.Inc()
		}
		return
	}
	dumpCounter.Inc()
}

func setServerGauges(target string, good, bad int) {
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			goodBkpServers.Set(float64(good))
			badBkpServers.Set(float64(bad))
		}
		return
	}
	goodServers.Set(float64(good))
	badServers.Set(float64(bad))
}

func setQueuedDumpsGauge(target string, count int) {
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			queuedBkpDumps.Set(float64(count))
		}
		return
	}
	queuedDumps.Set(float64(count))
}

func recordLastSent(target string) {
	now := float64(time.Now().Unix())
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			lastBkpSentUnix.Set(now)
		}
		return
	}
	lastSentUnix.Set(now)
}

func setSendQueueGauge(target string, length int64) {
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			sendBkpQueue.Set(float64(length))
		}
		return
	}
	sendQueue.Set(float64(length))
}

var journalDirBytes = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_journal_dir_bytes",
		Help: "Total bytes used by journal directory (wal + ack)",
	})

func setJournalPendingGauge(j *Journal) {
	if j == nil {
		return
	}
	n, err := j.PendingCount()
	if err != nil {
		return
	}
	journalPending.Set(float64(n))
	if b, err := j.DirBytes(); err == nil {
		journalDirBytes.Set(float64(b))
	}
}

func setDumpDirBytesGauge(target string, bytes int64) {
	if isBackupTarget(target) {
		if backupMetricsEnabled {
			dumpBkpDirBytes.Set(float64(bytes))
		}
		return
	}
	dumpDirBytes.Set(float64(bytes))
}

// InitMetrics registers Prometheus collectors. Backup metrics are omitted when backup is disabled.
func InitMetrics(prefix string, backupEnabled bool) {
	backupMetricsEnabled = backupEnabled
	prometheus.DefaultRegisterer = prometheus.WrapRegistererWithPrefix(prefix, prometheus.DefaultRegisterer)

	prometheus.MustRegister(pushCounter)
	prometheus.MustRegister(sentCounter)
	prometheus.MustRegister(dumpCounter)
	prometheus.MustRegister(queuedDumps)
	prometheus.MustRegister(goodServers)
	prometheus.MustRegister(badServers)
	prometheus.MustRegister(sendQueue)
	prometheus.MustRegister(lastSentUnix)
	prometheus.MustRegister(dumpDirBytes)

	if backupEnabled {
		prometheus.MustRegister(sentBkpCounter)
		prometheus.MustRegister(dumpBkpCounter)
		prometheus.MustRegister(queuedBkpDumps)
		prometheus.MustRegister(goodBkpServers)
		prometheus.MustRegister(badBkpServers)
		prometheus.MustRegister(sendBkpQueue)
		prometheus.MustRegister(lastBkpSentUnix)
		prometheus.MustRegister(dumpBkpDirBytes)
	}
}

// RegisterJournalMetrics registers journal gauge (call when journal is enabled).
func RegisterJournalMetrics() {
	prometheus.MustRegister(journalPending)
	prometheus.MustRegister(journalDirBytes)
}
