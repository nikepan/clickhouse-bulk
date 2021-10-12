package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var pushCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_received_count",
		Help: "Received requests count from launch",
	})
var sentCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_sent_count",
		Help: "Sent request count from launch",
	})
var dumpCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_dump_count",
		Help: "Dumps saved from launch",
	})
var goodServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_good_servers",
		Help: "Actual good servers count",
	})
var badServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bad_servers",
		Help: "Actual count of bad servers",
	})

var queuedDumps = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_queued_dumps",
		Help: "Actual dump files id directory",
	})

// InitMetrics - init prometheus metrics
func InitMetrics(prefix string) {
	prometheus.DefaultRegisterer = prometheus.WrapRegistererWithPrefix(prefix, prometheus.DefaultRegisterer)

	prometheus.MustRegister(pushCounter)
	prometheus.MustRegister(sentCounter)
	prometheus.MustRegister(dumpCounter)
	prometheus.MustRegister(queuedDumps)
	prometheus.MustRegister(goodServers)
	prometheus.MustRegister(badServers)
}
