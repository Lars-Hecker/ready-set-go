package config

import (
	"os"
	"strconv"
	"time"
)

type S3Config struct {
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	URLLifetime     time.Duration
}

func S3ConfigFromEnv() S3Config {
	lifetime := 15 * time.Minute
	if v := os.Getenv("S3_URL_LIFETIME_MINUTES"); v != "" {
		if mins, err := strconv.Atoi(v); err == nil && mins > 0 {
			lifetime = time.Duration(mins) * time.Minute
		}
	}
	return S3Config{
		Bucket:          os.Getenv("S3_BUCKET"),
		Region:          os.Getenv("S3_REGION"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		URLLifetime:     lifetime,
	}
}
