package ai

import "errors"

var (
	// ErrQuotaExceeded is returned when the user has exceeded their token quota.
	ErrQuotaExceeded = errors.New("ai: token quota exceeded")

	// ErrGenerationFailed is returned when text generation fails.
	ErrGenerationFailed = errors.New("ai: generation failed")
)
