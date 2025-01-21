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
	tlsServerName  string   `json:"tls_server_name"`
	tlsSkipVerify  bool     `json:"insecure_tls_skip_verify"`
	DownTimeout    int      `json:"down_timeout"`
	ConnectTimeout int      `json:"connect_timeout"`
}

// Config stores config data
type Config struct {
	Listen            string           `json:"listen"`
	Clickhouse        clickhouseConfig `json:"clickhouse"`
	FlushCount        int              `json:"flush_count"`
	FlushInterval     int              `json:"flush_interval"`
	CleanInterval     int              `json:"clean_interval"`
	RemoveQueryID     bool             `json:"remove_query_id"`
	DumpCheckInterval int              `json:"dump_check_interval"`
	DumpDir           string           `json:"dump_dir"`
	Debug             bool             `json:"debug"`
	LogQueries        bool             `json:"log_queries"`
	MetricsPrefix     string           `json:"metrics_prefix"`
	UseTLS            bool             `json:"use_tls"`
	TLSCertFile       string           `json:"tls_cert_file"`
	TLSKeyFile        string           `json:"tls_key_file"`
}

func defaultConfig() Config {
	return Config{
		Listen:            ":8124",
		FlushCount:        10000,
		FlushInterval:     1000,
		CleanInterval:     0,
		RemoveQueryID:     true,
		DumpCheckInterval: 300,
		DumpDir:           "dumps",
		Debug:             false,
		LogQueries:        false,
		MetricsPrefix:     "",
		UseTLS:            false,
		TLSCertFile:       "",
		TLSKeyFile:        "",
		Clickhouse: clickhouseConfig{
			DownTimeout:    60,
			ConnectTimeout: 10,
			Servers:        []string{"http://127.0.0.1:8123"},
		},
	}
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

func readEnvInt(name string, value *int) {
	s := os.Getenv(name)
	if s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			log.Printf("ERROR: Wrong %+v env: %+v\n", name, err)
		}
		*value = v
	}
}

func readEnvBool(name string, value *bool) {
    s := os.Getenv(name)
    if s != "" {
        v, err := strconv.ParseBool(s)
        if err != nil {
            log.Printf("ERROR: Wrong %+v env: %+v\n", name, err)
        } else {
            *value = v
        }
    }
}

func readEnvString(name string, value *string) {
	s := os.Getenv(name)
	if s != "" {
		*value = s
	}
}


// ReadConfig init config data
func ReadConfig(configFile string) (Config, error) {
	// Start with default values
	cnf := defaultConfig()

	// Load the config file if it exists
	if _, err := os.Stat(configFile); err == nil {
		if err := ReadJSON(configFile, &cnf); err != nil {
			return Config{}, err
		}
	} else if !os.IsNotExist(err) {
		// Return other errors (e.g., permission issues)
		return Config{}, err
	}

	// Apply environment variable overrides
	readEnvBool("CLICKHOUSE_BULK_DEBUG", &cnf.Debug)
	readEnvInt("CLICKHOUSE_FLUSH_COUNT", &cnf.FlushCount)
	readEnvInt("CLICKHOUSE_FLUSH_INTERVAL", &cnf.FlushInterval)
	readEnvInt("CLICKHOUSE_CLEAN_INTERVAL", &cnf.CleanInterval)
	readEnvBool("CLICKHOUSE_REMOVE_QUERY_ID", &cnf.RemoveQueryID)
	readEnvInt("DUMP_CHECK_INTERVAL", &cnf.DumpCheckInterval)
	readEnvInt("CLICKHOUSE_DOWN_TIMEOUT", &cnf.Clickhouse.DownTimeout)
	readEnvInt("CLICKHOUSE_CONNECT_TIMEOUT", &cnf.Clickhouse.ConnectTimeout)
	readEnvBool("CLICKHOUSE_INSECURE_TLS_SKIP_VERIFY", &cnf.Clickhouse.tlsSkipVerify)
	readEnvString("METRICS_PREFIX", &cnf.MetricsPrefix)
	readEnvBool("LOG_QUERIES", &cnf.LogQueries)

	serversList := os.Getenv("CLICKHOUSE_SERVERS")
	if serversList != "" {
		cnf.Clickhouse.Servers = strings.Split(serversList, ",")
	}

	log.Printf("use servers: %+v", strings.Join(cnf.Clickhouse.Servers, ", "))
	return cnf, nil
}
