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
