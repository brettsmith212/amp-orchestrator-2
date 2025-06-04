package config

import (
	"os"
)

type Config struct {
	Port      string
	AmpBinary string
	LogDir    string
}

func Load() *Config {
	return &Config{
		Port:      getEnv("PORT", "8080"),
		AmpBinary: getEnv("AMP_BINARY", "amp"),
		LogDir:    getEnv("LOG_DIR", "./logs"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
