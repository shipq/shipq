package llm

import (
	"testing"
	"time"
)

func TestParseRetryAfter_Header(t *testing.T) {
	d := ParseRetryAfter("5", "")
	if d != 5*time.Second {
		t.Errorf("got %v, want 5s", d)
	}
}

func TestParseRetryAfter_HeaderFloat(t *testing.T) {
	d := ParseRetryAfter("4.5", "")
	if d != 4500*time.Millisecond {
		t.Errorf("got %v, want 4.5s", d)
	}
}

func TestParseRetryAfter_HeaderWithWhitespace(t *testing.T) {
	d := ParseRetryAfter("  3  ", "")
	if d != 3*time.Second {
		t.Errorf("got %v, want 3s", d)
	}
}

func TestParseRetryAfter_BodyFallback(t *testing.T) {
	body := "Rate limit reached. Please try again in 4.5s."
	d := ParseRetryAfter("", body)
	if d != 4500*time.Millisecond {
		t.Errorf("got %v, want 4.5s", d)
	}
}

func TestParseRetryAfter_BodyRetryIn(t *testing.T) {
	body := "Too many requests. Retry in 10s."
	d := ParseRetryAfter("", body)
	if d != 10*time.Second {
		t.Errorf("got %v, want 10s", d)
	}
}

func TestParseRetryAfter_HeaderTakesPrecedence(t *testing.T) {
	d := ParseRetryAfter("2", "Please try again in 10s.")
	if d != 2*time.Second {
		t.Errorf("got %v, want 2s (header should take precedence)", d)
	}
}

func TestParseRetryAfter_NeitherPresent(t *testing.T) {
	d := ParseRetryAfter("", "some other error message")
	if d != 0 {
		t.Errorf("got %v, want 0", d)
	}
}

func TestParseRetryAfter_InvalidHeader(t *testing.T) {
	d := ParseRetryAfter("not-a-number", "Please try again in 3s.")
	if d != 3*time.Second {
		t.Errorf("got %v, want 3s (should fall back to body)", d)
	}
}

func TestRateLimitError_Error(t *testing.T) {
	err := &RateLimitError{
		StatusCode: 429,
		RetryAfter: 5 * time.Second,
		Message:    "rate limit exceeded",
	}
	s := err.Error()
	if s != "HTTP 429: rate limit exceeded (retry after 5s)" {
		t.Errorf("got %q", s)
	}
}

func TestRateLimitError_ErrorNoRetryAfter(t *testing.T) {
	err := &RateLimitError{
		StatusCode: 429,
		Message:    "rate limit exceeded",
	}
	s := err.Error()
	if s != "HTTP 429: rate limit exceeded" {
		t.Errorf("got %q", s)
	}
}

func TestParseRetryAfter_AnthropicGenericMessage(t *testing.T) {
	// Anthropic's 429 body says "Your account has hit a rate limit." with no
	// "try again in Xs" hint. Without a Retry-After header, we get 0.
	d := ParseRetryAfter("", "rate_limit_error: Your account has hit a rate limit.")
	if d != 0 {
		t.Errorf("got %v, want 0 (Anthropic message has no retry hint)", d)
	}
}

func TestParseRetryAfter_AnthropicWithHeader(t *testing.T) {
	// With Anthropic's retry-after header, the header takes effect even though
	// the body has no parseable duration.
	d := ParseRetryAfter("7", "rate_limit_error: Your account has hit a rate limit.")
	if d != 7*time.Second {
		t.Errorf("got %v, want 7s", d)
	}
}

func TestParseRetryAfter_OpenAIFullMessage(t *testing.T) {
	// Real OpenAI 429 error message with embedded wait time.
	body := "tokens: Rate limit reached for gpt-5-search-api in organization org-xxx on tokens per min (TPM): Limit 45000, Used 45000, Requested 3375. Please try again in 4.5s."
	d := ParseRetryAfter("", body)
	if d != 4500*time.Millisecond {
		t.Errorf("got %v, want 4.5s", d)
	}
}
