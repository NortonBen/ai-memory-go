package utils

import (
	"errors"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "rate limit", err: errors.New("Rate Limit exceeded"), want: true},
		{name: "timeout", err: errors.New("request timeout"), want: true},
		{name: "server 503", err: errors.New("upstream returned 503"), want: true},
		{name: "connection reset", err: errors.New("connection reset by peer"), want: true},
		{name: "non retryable", err: errors.New("validation failed"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.want {
				t.Fatalf("IsRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}
