package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// MustGetEnv returns the environment variable value or panics if not set.
// Use this for required configuration values.
func MustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

// GetEnvOrDefault returns the environment variable value or the default if not set.
func GetEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// MustGetEnvAsInt returns the environment variable as int or panics.
func MustGetEnvAsInt(key string) int {
	val := MustGetEnv(key)
	i, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s must be an integer: %v", key, err))
	}
	return i
}

// GetEnvAsIntOrDefault returns the environment variable as int or the default.
func GetEnvAsIntOrDefault(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// GetEnvAsDurationOrDefault returns the environment variable as duration or the default.
func GetEnvAsDurationOrDefault(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

// GetEnvAsBoolOrDefault returns the environment variable as bool or the default.
func GetEnvAsBoolOrDefault(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

// envCollector collects missing required environment variables for batch validation.
type envCollector struct {
	missing []string
}

// Require returns the env var value and tracks if it's missing.
func (c *envCollector) Require(key string) string {
	val := os.Getenv(key)
	if val == "" {
		c.missing = append(c.missing, key)
	}
	return val
}

// Validate returns an error if any required variables are missing.
func (c *envCollector) Validate() error {
	if len(c.missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", c.missing)
	}
	return nil
}

// NewEnvCollector creates a new environment variable collector for batch validation.
func NewEnvCollector() *envCollector {
	return &envCollector{}
}

// IsProduction checks if the current environment is production.
// Checks ENV environment variable.
// Production values: "production", "prod", "prd"
func IsProduction() bool {
	env := GetEnvOrDefault("ENV", "development")
	switch env {
	case "production", "prod", "prd":
		return true
	default:
		return false
	}
}

// IsDevelopment checks if the current environment is development.
func IsDevelopment() bool {
	env := GetEnvOrDefault("ENV", "development")
	return env == "development" || env == "dev"
}
