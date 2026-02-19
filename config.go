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
	// Leave at 0 (the default) for streaming calls — http.Client.Timeout is a
	// total round-trip deadline that fires while the response body is still being
	// read, which breaks SSE streams that run longer than the timeout.
	// Use the request context to enforce deadlines on streaming calls instead.
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
