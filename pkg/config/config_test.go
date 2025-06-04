package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear any existing env vars
	clearTestEnvVars()
	defer clearTestEnvVars()

	config := Load()

	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "amp", config.AmpBinary)
	assert.Equal(t, "./logs", config.LogDir)
}

func TestLoad_CustomValues(t *testing.T) {
	clearTestEnvVars()
	defer clearTestEnvVars()

	// Set custom env vars
	os.Setenv("PORT", "9090")
	os.Setenv("AMP_BINARY", "/usr/local/bin/amp")
	os.Setenv("LOG_DIR", "/tmp/logs")

	config := Load()

	assert.Equal(t, "9090", config.Port)
	assert.Equal(t, "/usr/local/bin/amp", config.AmpBinary)
	assert.Equal(t, "/tmp/logs", config.LogDir)
}

func TestLoad_PartialCustomValues(t *testing.T) {
	clearTestEnvVars()
	defer clearTestEnvVars()

	// Set only PORT
	os.Setenv("PORT", "3000")

	config := Load()

	assert.Equal(t, "3000", config.Port)
	assert.Equal(t, "amp", config.AmpBinary)   // default
	assert.Equal(t, "./logs", config.LogDir)  // default
}

func TestLoad_EmptyValues(t *testing.T) {
	clearTestEnvVars()
	defer clearTestEnvVars()

	// Set empty values
	os.Setenv("PORT", "")
	os.Setenv("AMP_BINARY", "")
	os.Setenv("LOG_DIR", "")

	config := Load()

	// Empty values should fall back to defaults
	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "amp", config.AmpBinary)
	assert.Equal(t, "./logs", config.LogDir)
}

func TestGetEnv(t *testing.T) {
	clearTestEnvVars()
	defer clearTestEnvVars()

	// Test with no env var set
	result := getEnv("NONEXISTENT", "default")
	assert.Equal(t, "default", result)

	// Test with env var set
	os.Setenv("TEST_VAR", "test_value")
	result = getEnv("TEST_VAR", "default")
	assert.Equal(t, "test_value", result)

	// Test with empty env var
	os.Setenv("EMPTY_VAR", "")
	result = getEnv("EMPTY_VAR", "default")
	assert.Equal(t, "default", result)
}

func clearTestEnvVars() {
	os.Unsetenv("PORT")
	os.Unsetenv("AMP_BINARY")
	os.Unsetenv("LOG_DIR")
	os.Unsetenv("TEST_VAR")
	os.Unsetenv("EMPTY_VAR")
}
