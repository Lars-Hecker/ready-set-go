package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Require returns the value of the environment variable or exits if not set.
func Require(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("env required", "key", key)
		os.Exit(1)
	}
	return v
}

// Get returns the value of the environment variable or the fallback if not set.
func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetInt returns the integer value of the environment variable or the fallback if not set or invalid.
func GetInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

// GetDuration returns the duration value of the environment variable or the fallback if not set or invalid.
// The value should be in a format parseable by time.ParseDuration (e.g., "5m", "1h30m").
func GetDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
