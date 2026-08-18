package main

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureLog - collects everything written to the standard logger in fn
func captureLog(fn func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	fn()
	return buf.String()
}

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
	assert.Equal(t, 60, cnf.Clickhouse.SendTimeout)
	assert.Equal(t, "", cnf.MaxBodySize)
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

func TestEnvOverrideInvalidInt(t *testing.T) {
	os.Setenv("CLICKHOUSE_FLUSH_COUNT", "notanumber")
	defer os.Unsetenv("CLICKHOUSE_FLUSH_COUNT")

	cnf, err := ReadConfig("non_existent_config.json")
	assert.Nil(t, err)

	// invalid env value must not override the default with zero
	assert.Equal(t, 10000, cnf.FlushCount)
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

func TestRedactURL(t *testing.T) {
	assert.Equal(t, "http://user:xxxxx@127.0.0.1:8123", RedactURL("http://user:secretpass@127.0.0.1:8123"))
	assert.Equal(t, "http://127.0.0.1:8123", RedactURL("http://127.0.0.1:8123"))
	assert.Equal(t, "", RedactURL(""))
	// an unparsable url may still hold credentials, they must not pass through
	assert.NotContains(t, RedactURL("http://user:secret pass@127.0.0.1:8123"), "secret")
}

func TestRedactQuery(t *testing.T) {
	assert.Equal(t, "user=default&password=xxxxx&query=SELECT+1", RedactQuery("user=default&password=secretpass&query=SELECT+1"))
	assert.Equal(t, "password=xxxxx", RedactQuery("password=secretpass"))
	assert.Equal(t, "query=SELECT+1", RedactQuery("query=SELECT+1"))
	assert.Equal(t, "", RedactQuery(""))
}

func TestReadConfigDoesNotLogCredentials(t *testing.T) {
	configContent := `{"clickhouse": {"servers": ["http://user:secretpass@127.0.0.1:8123"]}}`
	tmpFile, err := os.CreateTemp("", "test_creds_config_*.json")
	assert.Nil(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.Nil(t, err)
	tmpFile.Close()

	out := captureLog(func() {
		cnf, err := ReadConfig(tmpFile.Name())
		assert.Nil(t, err)
		// the config itself must keep the credentials, only the log is redacted
		assert.Equal(t, []string{"http://user:secretpass@127.0.0.1:8123"}, cnf.Clickhouse.Servers)
	})

	assert.Contains(t, out, "127.0.0.1:8123")
	assert.NotContains(t, out, "secretpass")
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
