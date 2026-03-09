package ai

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	GoogleAPIKey  string
	DefaultModel  string
	DefaultQuota  int64 // Monthly token quota per user
	ReserveTokens int64 // Tokens to reserve per request
}

func ConfigFromEnv() Config {
	cfg := Config{
		GoogleAPIKey:  os.Getenv("GOOGLE_API_KEY"),
		DefaultModel:  os.Getenv("AI_DEFAULT_MODEL"),
		DefaultQuota:  100000,
		ReserveTokens: 4096,
	}

	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "gemini-2.0-flash"
	}

	if v := os.Getenv("AI_DEFAULT_QUOTA"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.DefaultQuota = n
		}
	}

	if v := os.Getenv("AI_RESERVE_TOKENS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.ReserveTokens = n
		}
	}

	return cfg
}

func (c Config) Validate() error {
	if c.GoogleAPIKey == "" {
		return errors.New("GOOGLE_API_KEY is required")
	}
	return nil
}
