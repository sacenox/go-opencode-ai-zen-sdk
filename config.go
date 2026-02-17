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
	APIKey     string
	BaseURL    string
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

	if c.Timeout == 0 {
		c.Timeout = 60 * time.Second
	}

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
