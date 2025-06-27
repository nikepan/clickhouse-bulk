package main

import (
	"flag"
	"log"
	"os"
)

var version = "unknown"
var commit = "unknown"
var date = "unknown"

func main() {

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	configFile := flag.String("config", "config.json", "config file (json)")

	flag.Parse()

	if flag.Arg(0) == "version" {
		log.Printf("clickhouse-bulk v%s (commit: %s, built: %s)\n", version, commit, date)
		return
	}

	log.Printf("Starting clickhouse-bulk v%s (commit: %s, built: %s)\n", version, commit, date)

	cnf, err := ReadConfig(*configFile)
	if err != nil {
		log.Fatalf("ERROR: %+v\n", err)
	}
	RunServer(cnf)
}
