package llm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RateLimitError is returned by providers when the API responds with HTTP 429.
// It carries the parsed Retry-After duration so callers can wait the right
// amount of time before retrying.
type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("HTTP %d: %s (retry after %s)", e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// retryAfterRegexp matches patterns like "try again in 4.5s" or "retry in 10s"
// that OpenAI embeds in error message bodies.
var retryAfterRegexp = regexp.MustCompile(`(?i)(?:try|retry)\s+(?:again\s+)?in\s+(\d+(?:\.\d+)?)\s*s`)

// ParseRetryAfter extracts a retry-after duration from a combination of the
// HTTP Retry-After header and the error message body.
//
// Priority:
//  1. Retry-After header (integer seconds per RFC 7231)
//  2. "try again in X.Xs" pattern in the message body
//  3. Zero (caller should use a fallback)
func ParseRetryAfter(header string, body string) time.Duration {
	header = strings.TrimSpace(header)
	if header != "" {
		if secs, err := strconv.ParseFloat(header, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
	}

	if m := retryAfterRegexp.FindStringSubmatch(body); len(m) >= 2 {
		if secs, err := strconv.ParseFloat(m[1], 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
	}

	return 0
}
