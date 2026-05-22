package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const sampleConfig = "config.sample.json"

type clickhouseConfig struct {
	Servers        []string `json:"servers"`
	QueryParams    string   `json:"query_params"`
	TLSServerName  string   `json:"tls_server_name"`
	TLSSkipVerify  bool     `json:"insecure_tls_skip_verify"`
	DownTimeout    int `json:"down_timeout"`
	ConnectTimeout int `json:"connect_timeout"`
	SendMaxRPS     int `json:"send_max_rps"`
	SendMaxBurst   int `json:"send_max_burst"`
}

// Config stores config data
type Config struct {
	Listen            string            `json:"listen"`
	Clickhouse        clickhouseConfig  `json:"clickhouse"`
	ClickhouseBackup  *clickhouseConfig `json:"clickhouse-backup"`
	FlushCount        int               `json:"flush_count"`
	FlushInterval     int               `json:"flush_interval"`
	CleanInterval     int               `json:"clean_interval"`
	RemoveQueryID     bool              `json:"remove_query_id"`
	DumpCheckInterval    int `json:"dump_check_interval"`
	BkpDumpCheckInterval int `json:"bkp_dump_check_interval"`
	DumpReplayBatch      int `json:"dump_replay_batch"`
	MaxDumpFiles         int `json:"max_dump_files"`
	DumpDir              string `json:"dump_dir"`
	BkpDumpDir           string `json:"bkp_dump_dir"`
	JournalDir           string `json:"journal_dir"`
	JournalFsync         bool   `json:"journal_fsync"`
	MaxJournalPending    int    `json:"max_journal_pending"`
	ShutdownDrainSec     int    `json:"shutdown_drain_sec"`
	Debug             bool              `json:"debug"`
	LogQueries        bool              `json:"log_queries"`
	MetricsPrefix     string            `json:"metrics_prefix"`
	UseTLS            bool              `json:"use_tls"`
	TLSCertFile       string            `json:"tls_cert_file"`
	TLSKeyFile        string            `json:"tls_key_file"`
}

// BackupEnabled reports whether live/backup dual-write mode is active.
func (c Config) BackupEnabled() bool {
	return c.ClickhouseBackup != nil && len(c.ClickhouseBackup.Servers) > 0
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
		ShutdownDrainSec:  60,
		JournalDir:        "",
		JournalFsync:      false,
		Debug:             false,
		LogQueries:        false,
		MetricsPrefix:     "",
		UseTLS:            false,
		TLSCertFile:       "",
		TLSKeyFile:        "",
		Clickhouse: clickhouseConfig{
			DownTimeout:    60,
			ConnectTimeout: 10,
			TLSServerName:  "",
			TLSSkipVerify:  false,
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

func splitTrimServers(list string) []string {
	parts := strings.Split(list, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeServerList(servers []string) []string {
	out := make([]string, 0, len(servers))
	for _, s := range servers {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func mergeQueryParams(base, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return base
	}
	if base == "" {
		return extra
	}
	return base + "&" + extra
}

// validateLocalDataDir normalizes admin-configured local paths and rejects traversal (..).
// Empty dir disables that feature (e.g. journal_dir "").
func validateLocalDataDir(dir, field string) (string, error) {
	if dir == "" {
		return "", nil
	}
	clean := filepath.Clean(dir)
	if clean == "." || clean == ".." {
		return "", fmt.Errorf("%s: invalid path %q", field, dir)
	}
	slash := filepath.ToSlash(dir)
	if strings.HasPrefix(slash, "../") || strings.Contains(slash, "/../") {
		return "", fmt.Errorf("%s: path must not contain '..'", field)
	}
	return clean, nil
}

func validateConfigDataDirs(cnf *Config) error {
	var err error
	if cnf.DumpDir, err = validateLocalDataDir(cnf.DumpDir, "dump_dir"); err != nil {
		return err
	}
	if cnf.BkpDumpDir, err = validateLocalDataDir(cnf.BkpDumpDir, "bkp_dump_dir"); err != nil {
		return err
	}
	if cnf.JournalDir, err = validateLocalDataDir(cnf.JournalDir, "journal_dir"); err != nil {
		return err
	}
	return nil
}

func validateClickhouseConfig(name string, ch clickhouseConfig) error {
	if len(ch.Servers) == 0 {
		return fmt.Errorf("%s: no servers configured", name)
	}
	for _, u := range ch.Servers {
		if strings.TrimSpace(u) == "" {
			return fmt.Errorf("%s: empty server URL in servers list", name)
		}
	}
	return nil
}

func ensureBackupConfig(cnf *Config) *clickhouseConfig {
	if cnf.ClickhouseBackup == nil {
		cnf.ClickhouseBackup = &clickhouseConfig{
			DownTimeout:    cnf.Clickhouse.DownTimeout,
			ConnectTimeout: cnf.Clickhouse.ConnectTimeout,
			TLSServerName:  cnf.Clickhouse.TLSServerName,
			TLSSkipVerify:  cnf.Clickhouse.TLSSkipVerify,
		}
	}
	return cnf.ClickhouseBackup
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
	readEnvInt("BKP_DUMP_CHECK_INTERVAL", &cnf.BkpDumpCheckInterval)
	readEnvInt("DUMP_REPLAY_BATCH", &cnf.DumpReplayBatch)
	readEnvInt("MAX_DUMP_FILES", &cnf.MaxDumpFiles)
	readEnvString("DUMP_DIR", &cnf.DumpDir)
	readEnvString("CLICKHOUSE_BKP_DUMP_DIR", &cnf.BkpDumpDir)
	readEnvInt("SHUTDOWN_DRAIN_SEC", &cnf.ShutdownDrainSec)
	readEnvString("JOURNAL_DIR", &cnf.JournalDir)
	readEnvBool("JOURNAL_FSYNC", &cnf.JournalFsync)
	readEnvInt("MAX_JOURNAL_PENDING", &cnf.MaxJournalPending)
	readEnvInt("CLICKHOUSE_DOWN_TIMEOUT", &cnf.Clickhouse.DownTimeout)
	readEnvInt("CLICKHOUSE_CONNECT_TIMEOUT", &cnf.Clickhouse.ConnectTimeout)
	readEnvInt("CLICKHOUSE_SEND_MAX_RPS", &cnf.Clickhouse.SendMaxRPS)
	readEnvInt("CLICKHOUSE_SEND_MAX_BURST", &cnf.Clickhouse.SendMaxBurst)
	readEnvString("CLICKHOUSE_TLS_SERVER_NAME", &cnf.Clickhouse.TLSServerName)
	readEnvBool("CLICKHOUSE_INSECURE_TLS_SKIP_VERIFY", &cnf.Clickhouse.TLSSkipVerify)
	readEnvString("METRICS_PREFIX", &cnf.MetricsPrefix)
	readEnvBool("LOG_QUERIES", &cnf.LogQueries)

	serversList := os.Getenv("CLICKHOUSE_SERVERS")
	if serversList != "" {
		cnf.Clickhouse.Servers = splitTrimServers(serversList)
	}
	cnf.Clickhouse.Servers = normalizeServerList(cnf.Clickhouse.Servers)

	backupServers := os.Getenv("CLICKHOUSE_BACKUP_SERVERS")
	if backupServers != "" {
		bkp := ensureBackupConfig(&cnf)
		bkp.Servers = splitTrimServers(backupServers)
	}
	if cnf.BackupEnabled() {
		bkp := ensureBackupConfig(&cnf)
		bkp.Servers = normalizeServerList(bkp.Servers)
		readEnvInt("CLICKHOUSE_BACKUP_DOWN_TIMEOUT", &bkp.DownTimeout)
		readEnvInt("CLICKHOUSE_BACKUP_CONNECT_TIMEOUT", &bkp.ConnectTimeout)
		readEnvString("CLICKHOUSE_BACKUP_TLS_SERVER_NAME", &bkp.TLSServerName)
		readEnvBool("CLICKHOUSE_BACKUP_INSECURE_TLS_SKIP_VERIFY", &bkp.TLSSkipVerify)
		readEnvString("CLICKHOUSE_BACKUP_QUERY_PARAMS", &bkp.QueryParams)
		readEnvInt("CLICKHOUSE_BACKUP_SEND_MAX_RPS", &bkp.SendMaxRPS)
		readEnvInt("CLICKHOUSE_BACKUP_SEND_MAX_BURST", &bkp.SendMaxBurst)
	}

	if err := validateClickhouseConfig("clickhouse", cnf.Clickhouse); err != nil {
		return Config{}, err
	}
	if cnf.BackupEnabled() {
		if err := validateClickhouseConfig("clickhouse-backup", *cnf.ClickhouseBackup); err != nil {
			return Config{}, err
		}
	}
	if cnf.ShutdownDrainSec <= 0 {
		cnf.ShutdownDrainSec = 60
	}
	if err := validateConfigDataDirs(&cnf); err != nil {
		return Config{}, err
	}

	log.Printf("Using servers: %+v", strings.Join(cnf.Clickhouse.Servers, ", "))
	if cnf.BackupEnabled() {
		log.Printf("Using backup servers: %+v", strings.Join(cnf.ClickhouseBackup.Servers, ", "))
	}
	return cnf, nil
}
