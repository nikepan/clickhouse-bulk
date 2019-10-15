package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var pushCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_received_count",
	})
var sentCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_sent_count",
	})
var dumpCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "ch_dump_count",
	})
var goodServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_good_servers",
	})
var badServers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_bad_servers",
	})

var queuedDumps = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "ch_queued_dumps",
	})

// InitMetrics - init prometheus metrics
func InitMetrics() {

	prometheus.MustRegister(pushCounter)
	prometheus.MustRegister(sentCounter)
	prometheus.MustRegister(dumpCounter)
	prometheus.MustRegister(goodServers)
	prometheus.MustRegister(badServers)
}
