package zen

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://opencode.ai/zen/v1"

type RetryConfig struct {
	MaxRetries           int
	RetryOnNonIdempotent bool
	Backoff              func(attempt int) time.Duration
}

type Config struct {
	APIKey  string
	BaseURL string
	// Timeout sets http.Client.Timeout on the internal HTTP client.
	//
	// WARNING: http.Client.Timeout is a total round-trip deadline — it starts
	// when the request is sent and fires while the response body is still being
	// read.  Setting a non-zero value WILL kill SSE/streaming responses that
	// run longer than the deadline with "context deadline exceeded".
	//
	// Leave at 0 (the default) for streaming calls and enforce deadlines via
	// the request context instead (e.g. context.WithTimeout / WithDeadline).
	//
	// The internal transport always applies a 30-second
	// ResponseHeaderTimeout independently of this field, protecting against
	// servers that accept the connection but never send headers.  This field
	// has no effect when HTTPClient is supplied by the caller.
	Timeout    time.Duration
	UserAgent  string
	HTTPClient *http.Client
	Retry      RetryConfig
}

func (c *Config) applyDefaults() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("zen: API key is required")
	}

	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = defaultBaseURL
	}

	c.BaseURL = strings.TrimRight(c.BaseURL, "/")

	if strings.TrimSpace(c.UserAgent) == "" {
		c.UserAgent = "go-opencode-zen-sdk/0.1"
	}

	if c.Retry.Backoff == nil {
		c.Retry.Backoff = func(attempt int) time.Duration {
			base := 200 * time.Millisecond
			if attempt <= 0 {
				return base
			}
			return base * time.Duration(1<<attempt)
		}
	}

	return nil
}
