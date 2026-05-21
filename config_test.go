package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadConfig(t *testing.T) {
	// Test with a non-existent file (should use defaults)
	cnf, err := ReadConfig("non_existent_config.json")
	assert.Nil(t, err)
	assert.NotNil(t, cnf)
	assert.Equal(t, ":8124", cnf.Listen)
	assert.Equal(t, 10000, cnf.FlushCount)
	assert.Equal(t, 1000, cnf.FlushInterval)
	assert.Equal(t, 300, cnf.DumpCheckInterval)
	assert.True(t, cnf.RemoveQueryID)
	assert.Equal(t, []string{"http://127.0.0.1:8123"}, cnf.Clickhouse.Servers)
}

func TestDefaultValues(t *testing.T) {
	// Simulate an empty config file
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	assert.Nil(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(`{}`)
	assert.Nil(t, err)
	tmpFile.Close()

	cnf, err := ReadConfig(tmpFile.Name())
	assert.Nil(t, err)

	// Verify defaults
	assert.Equal(t, ":8124", cnf.Listen)
	assert.Equal(t, 10000, cnf.FlushCount)
	assert.Equal(t, 1000, cnf.FlushInterval)
	assert.Equal(t, 0, cnf.CleanInterval)
	assert.False(t, cnf.Debug)
	assert.Equal(t, 60, cnf.Clickhouse.DownTimeout)
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("CLICKHOUSE_FLUSH_COUNT", "5000")
	os.Setenv("CLICKHOUSE_BULK_DEBUG", "true")
	defer os.Unsetenv("CLICKHOUSE_FLUSH_COUNT")
	defer os.Unsetenv("CLICKHOUSE_BULK_DEBUG")

	cnf, err := ReadConfig("config.sample.json")
	assert.Nil(t, err)

	// Verify overrides
	assert.Equal(t, 5000, cnf.FlushCount)
	assert.True(t, cnf.Debug)
}

func TestConfigFileStructure(t *testing.T) {
	// Check that the sample config file exists and is valid JSON
	_, err := os.Stat("config.sample.json")
	assert.Nil(t, err)

	var cnf Config
	err = ReadJSON("config.sample.json", &cnf)
	assert.Nil(t, err)
	assert.NotNil(t, cnf)

	// Validate required fields
	assert.NotEmpty(t, cnf.Listen)
	assert.Greater(t, cnf.FlushCount, 0)
	assert.Greater(t, len(cnf.Clickhouse.Servers), 0)
}

func TestBackupConfig(t *testing.T) {
	configContent := `{
		"dump_dir": "dumps",
		"bkp_dump_dir": "dumps-bkp",
		"clickhouse": {
			"servers": ["http://127.0.0.1:8123"]
		},
		"clickhouse-backup": {
			"servers": ["http://127.0.0.2:8123"]
		}
	}`
	tmpFile, err := os.CreateTemp("", "test_backup_config_*.json")
	assert.Nil(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.Nil(t, err)
	tmpFile.Close()

	cnf, err := ReadConfig(tmpFile.Name())
	assert.Nil(t, err)
	assert.True(t, cnf.BackupEnabled())
	assert.Equal(t, "dumps-bkp", cnf.BkpDumpDir)
	assert.Equal(t, []string{"http://127.0.0.2:8123"}, cnf.ClickhouseBackup.Servers)

	cnfNoBackup, err := ReadConfig("non_existent_config.json")
	assert.Nil(t, err)
	assert.False(t, cnfNoBackup.BackupEnabled())
}

func TestSplitTrimServers(t *testing.T) {
	assert.Equal(t, []string{"http://a", "http://b"}, splitTrimServers("http://a, http://b , "))
}

func TestMergeQueryParams(t *testing.T) {
	assert.Equal(t, "database=standby", mergeQueryParams("", "database=standby"))
	assert.Equal(t, "a=1&database=standby", mergeQueryParams("a=1", "database=standby"))
}

func TestBackupEnvOverrides(t *testing.T) {
	os.Setenv("CLICKHOUSE_BACKUP_SERVERS", "http://10.0.0.2:8123,http://10.0.0.3:8123")
	os.Setenv("CLICKHOUSE_BKP_DUMP_DIR", "/var/bkp-dumps")
	os.Setenv("CLICKHOUSE_BACKUP_DOWN_TIMEOUT", "120")
	os.Setenv("CLICKHOUSE_BACKUP_CONNECT_TIMEOUT", "5")
	os.Setenv("CLICKHOUSE_BACKUP_TLS_SERVER_NAME", "bkp.example.com")
	os.Setenv("CLICKHOUSE_BACKUP_INSECURE_TLS_SKIP_VERIFY", "true")
	defer func() {
		os.Unsetenv("CLICKHOUSE_BACKUP_SERVERS")
		os.Unsetenv("CLICKHOUSE_BKP_DUMP_DIR")
		os.Unsetenv("CLICKHOUSE_BACKUP_DOWN_TIMEOUT")
		os.Unsetenv("CLICKHOUSE_BACKUP_CONNECT_TIMEOUT")
		os.Unsetenv("CLICKHOUSE_BACKUP_TLS_SERVER_NAME")
		os.Unsetenv("CLICKHOUSE_BACKUP_INSECURE_TLS_SKIP_VERIFY")
	}()

	cnf, err := ReadConfig("non_existent_config.json")
	assert.Nil(t, err)
	assert.True(t, cnf.BackupEnabled())
	assert.Equal(t, "/var/bkp-dumps", cnf.BkpDumpDir)
	assert.Equal(t, []string{"http://10.0.0.2:8123", "http://10.0.0.3:8123"}, cnf.ClickhouseBackup.Servers)
	assert.Equal(t, 120, cnf.ClickhouseBackup.DownTimeout)
	assert.Equal(t, 5, cnf.ClickhouseBackup.ConnectTimeout)
	assert.Equal(t, "bkp.example.com", cnf.ClickhouseBackup.TLSServerName)
	assert.True(t, cnf.ClickhouseBackup.TLSSkipVerify)
}

func TestTLSConfig(t *testing.T) {
	// Create a temporary config file with TLS settings
	configContent := `{
		"clickhouse": {
			"servers": ["http://127.0.0.1:8123"],
			"tls_server_name": "example.com",
			"insecure_tls_skip_verify": true
		}
	}`
	tmpFile, err := os.CreateTemp("", "test_tls_config_*.json")
	assert.Nil(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.Nil(t, err)
	tmpFile.Close()

	// Load config from file
	cnf, err := ReadConfig(tmpFile.Name())
	assert.Nil(t, err)

	// Verify TLS settings from file
	assert.Equal(t, "example.com", cnf.Clickhouse.TLSServerName)
	assert.True(t, cnf.Clickhouse.TLSSkipVerify)

	// Override with environment variables
	os.Setenv("CLICKHOUSE_TLS_SERVER_NAME", "override.com")
	os.Setenv("CLICKHOUSE_INSECURE_TLS_SKIP_VERIFY", "false")
	defer os.Unsetenv("CLICKHOUSE_TLS_SERVER_NAME")
	defer os.Unsetenv("CLICKHOUSE_INSECURE_TLS_SKIP_VERIFY")

	cnf, err = ReadConfig(tmpFile.Name())
	assert.Nil(t, err)

	// Verify TLS settings from environment variables
	assert.Equal(t, "override.com", cnf.Clickhouse.TLSServerName)
	assert.False(t, cnf.Clickhouse.TLSSkipVerify)
}
