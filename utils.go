package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const sampleConfig = "config.sample.json"

var cnf Config

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

type Credentials struct {
	User string
	Pass string
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
		}
		*value = v
	}
}

func readEnvString(name string, value *string) {
	s := os.Getenv(name)
	if s != "" {
		*value = s
	}
}

// ReadConfig init config data
func ReadConfig(configFile string) error {
	cnf = Config{}
	err := ReadJSON(configFile, &cnf)
	if err != nil {
		log.Printf("INFO: Config file %+v not found. Used%+v\n", configFile, sampleConfig)
		err = ReadJSON(sampleConfig, &cnf)
		if err != nil {
			log.Printf("ERROR: read %+v failed\n", sampleConfig)
		}
	}

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
	log.Printf("use servers: %+v\n", strings.Join(cnf.Clickhouse.Servers, ", "))

	tlsServerName := os.Getenv("CLICKHOUSE_TLS_SERVER_NAME")
	if tlsServerName != "" {
		cnf.Clickhouse.tlsServerName = tlsServerName
	}

	return err
}

// getAuth retrieves auth credentials from request
// according to CH documentation @see "https://clickhouse.yandex/docs/en/interfaces/http/"
func getAuth(req *http.Request) *Credentials {
	// check X-ClickHouse- headers
	name := req.Header.Get("X-ClickHouse-User")
	pass := req.Header.Get("X-ClickHouse-Key")
	if name != "" {
		return &Credentials{
			User: name,
			Pass: pass,
		}
	}
	// if header is empty - check basicAuth
	if name, pass, ok := req.BasicAuth(); ok {
		return &Credentials{
			User: name,
			Pass: pass,
		}
	}
	// if basicAuth is empty - check URL params `user` and `password`
	params := req.URL.Query()
	if name := params.Get("user"); name != "" {
		pass := params.Get("password")
		return &Credentials{
			User: name,
			Pass: pass,
		}
	}
	// if still no credentials - treat it as `default` user request
	return &Credentials{
		User: "default",
		Pass: "",
	}
}

func CopyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	dst.Add("Connection", "Close")
}
