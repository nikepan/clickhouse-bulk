package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var version = "unknown"
var date = "unknown"

type clickhouseConfig struct {
	Servers     []string `json:"servers"`
	DownTimeout int      `json:"down_timeout"`
}

type config struct {
	Listen        string           `json:"listen"`
	Clickhouse    clickhouseConfig `json:"clickhouse"`
	FlushCount    int              `json:"flush_count"`
	FlushInterval int              `json:"flush_interval"`
	DumpDir       string           `json:"dump_dir"`
	Debug         bool             `json:"debug"`
}

// SafeQuit - safe prepare to quit
func SafeQuit(collect *Collector, sender Sender) {
	collect.FlushAll()
	if count := sender.Len(); count > 0 {
		log.Printf("Sending %+v tables\n", count)
	}
	for !sender.Empty() && !collect.Empty() {
		collect.WaitFlush()
	}
	collect.WaitFlush()
}

func main() {

	log.SetOutput(os.Stdout)

	configFile := flag.String("config", "config.json", "config file (json)")

	flag.Parse()

	if flag.Arg(0) == "version" {
		log.Printf("clickhouse-bulk ver. %+v (%+v)\n", version, date)
		return
	}

	cnf := config{}
	err := ReadJSON(*configFile, &cnf)
	if err != nil {
		log.Printf("Config file %+v not found. Use config.sample.json\n", *configFile)
		err := ReadJSON("config.sample.json", &cnf)
		if err != nil {
			log.Fatalf("Read config: %+v\n", err.Error())
		}
	}

	serversList := os.Getenv("CLICKHOUSE_SERVERS")
	if serversList != "" {
		cnf.Clickhouse.Servers = strings.Split(serversList, ",")
		log.Printf("use servers: %+v\n", serversList)
	}
	flushCount := os.Getenv("CLICKHOUSE_FLUSH_COUNT")
	if flushCount != "" {
		cnf.FlushCount, err = strconv.Atoi(flushCount)
		if err != nil {
			log.Fatalf("Wrong flush count env: %+v\n", err.Error())
		}
	}
	flushInterval := os.Getenv("CLICKHOUSE_FLUSH_INTERVAL")
	if flushInterval != "" {
		cnf.FlushInterval, err = strconv.Atoi(flushInterval)
		if err != nil {
			log.Fatalf("Wrong flush interval env: %+v\n", err.Error())
		}
	}

	dumper := new(FileDumper)
	dumper.Path = cnf.DumpDir
	sender := NewClickhouse(cnf.Clickhouse.DownTimeout)
	sender.Dumper = dumper
	for _, url := range cnf.Clickhouse.Servers {
		sender.AddServer(url)
	}

	collect := NewCollector(sender, cnf.FlushCount, cnf.FlushInterval)

	// send collected data on SIGTERM and exit
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	srv := InitServer(cnf.Listen, collect, cnf.Debug)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		for {
			_ = <-signals
			log.Printf("STOP signal\n")
			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("Shutdown error %+v\n", err)
				SafeQuit(collect, sender)
				os.Exit(1)
			}
		}
	}()

	err = srv.Start()
	if err != nil {
		log.Printf("ListenAndServe: %+v\n", err)
		SafeQuit(collect, sender)
		os.Exit(1)
	}
}
