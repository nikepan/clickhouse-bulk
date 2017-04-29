package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type clickhouseConfig struct {
	Servers     []string `json:"servers"`
	DownTimeout int      `json:"down_timeout"`
}

type config struct {
	Listen        string           `json:"listen"`
	Clickhouse    clickhouseConfig `json:"clickhouse"`
	FlushCount    int              `json:"flush_count"`
	FlushInterval int              `json:"flush_interval"`
	Debug         bool             `json:"debug"`
}

func main() {

	log.SetOutput(os.Stdout)

	configFile := flag.String("config", "config.json", "config file (json)")

	flag.Parse()
	cnf := config{}
	err := ReadJSON(*configFile, &cnf)
	if err != nil {
		log.Printf("Config file %+v not found. Use config.sample.json", *configFile)
		err := ReadJSON("config.sample.json", &cnf)
		if err != nil {
			log.Fatalf("Read config: %+v\n", err.Error())
		}
	}

	sender := NewClickhouse(cnf.Clickhouse.DownTimeout)
	for _, url := range cnf.Clickhouse.Servers {
		sender.AddServer(url)
	}

	collect := NewCollector(sender, cnf.FlushCount, cnf.FlushInterval)

	// send collected data on SIGTERM and exit
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			_ = <-signals
			log.Printf("STOP signal")
			collect.FlushAll()
			if count := sender.Queue.Len(); count > 0 {
				log.Printf("Sending %+v tables", count)
			}
			for !sender.Queue.Empty() {
				time.Sleep(10)
			}
			os.Exit(1)
		}
	}()

	err = RunServer(cnf.Listen, collect, cnf.Debug)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
