// Package utils - Shared utility functions for extractor providers
package utils

import (
	"strings"
)

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Retry on rate limits, timeouts, and server errors
	retryablePatterns := []string{
		"rate limit",
		"rate_limit",
		"timeout",
		"503",
		"502",
		"500",
		"connection reset",
		"connection refused",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
