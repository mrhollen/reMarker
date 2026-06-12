package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Environment variable names.
const (
	EnvPassword     = "REMARKABLE_PASSWORD"
	EnvHost         = "REMARKABLE_HOST"
	EnvPort         = "REMARKABLE_PORT"
	EnvUser         = "REMARKABLE_USER"
	EnvSyncDir      = "REMARKER_SYNC_DIR"
	EnvSyncInterval = "SYNC_INTERVAL"
)

// Default values used when environment variables are not set or invalid.
const (
	defaultHost         = "10.11.99.1"
	defaultPort         = 22
	defaultUser         = "root"
	defaultSyncDir      = "./documents"
	defaultSyncInterval = 5 * time.Minute
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Host         string
	Port         int
	User         string
	Password     string
	SyncDir      string
	SyncInterval time.Duration
}

// Load reads configuration from environment variables and returns a Config
// with defaults applied where values are not set.
func Load() Config {
	return Config{
		Host:         envOr(EnvHost, defaultHost),
		Port:         loadPort(),
		User:         envOr(EnvUser, defaultUser),
		Password:     os.Getenv(EnvPassword),
		SyncDir:      envOr(EnvSyncDir, defaultSyncDir),
		SyncInterval: loadDuration(),
	}
}

// Validate checks that the configuration values are acceptable for operation.
func (c Config) Validate() error {
	if c.Password == "" {
		return fmt.Errorf("REMARKABLE_PASSWORD environment variable is required")
	}
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	return nil
}

// envOr returns the environment variable value or the default if the value
// is not set or is empty.
func envOr(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

// loadPort reads the port from the environment variable, falling back to
// defaultPort on parse error or negative values. Zero is kept as-is.
func loadPort() int {
	val := os.Getenv(EnvPort)
	if val == "" {
		return defaultPort
	}
	port, err := strconv.Atoi(val)
	if err != nil || port < 0 {
		return defaultPort
	}
	return port
}

// loadDuration reads the sync interval from the environment variable, falling
// back to defaultSyncInterval on parse error.
func loadDuration() time.Duration {
	val := os.Getenv(EnvSyncInterval)
	if val == "" {
		return defaultSyncInterval
	}
	dur, err := time.ParseDuration(val)
	if err != nil {
		return defaultSyncInterval
	}
	return dur
}
