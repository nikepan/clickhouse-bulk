package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
)

const sampleConfig = "config.sample.json"

type clickhouseConfig struct {
	Servers        []string `json:"servers"`
	DownTimeout    int      `json:"down_timeout"`
	ConnectTimeout int      `json:"connect_timeout"`
}

// Config stores config data
type Config struct {
	Listen            string           `json:"listen"`
	Clickhouse        clickhouseConfig `json:"clickhouse"`
	FlushCount        int              `json:"flush_count"`
	FlushInterval     int              `json:"flush_interval"`
	DumpCheckInterval int              `json:"dump_check_interval"`
	DumpDir           string           `json:"dump_dir"`
	Debug             bool             `json:"debug"`
}

// ReadJSON - read json file to struct
func ReadJSON(fn string, v interface{}) error {
	file, err := os.Open(fn)
	defer file.Close()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)
	return decoder.Decode(v)
}

// HasPrefix tests case insensitive whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.ToLower(s[0:len(prefix)]) == strings.ToLower(prefix)
}

// ReadConfig init config data
func ReadConfig(configFile string) (Config, error) {
	cnf := Config{}
	err := ReadJSON(configFile, &cnf)
	if err != nil {
		log.Printf("INFO: Config file %+v not found. Used%+v\n", configFile, sampleConfig)
		err = ReadJSON(sampleConfig, &cnf)
		if err != nil {
			log.Printf("ERROR: read %+v failed\n", sampleConfig)
		}
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

	serversList := os.Getenv("CLICKHOUSE_SERVERS")
	if serversList != "" {
		cnf.Clickhouse.Servers = strings.Split(serversList, ",")
		log.Printf("use servers: %+v\n", serversList)
	}

	return cnf, err
}
