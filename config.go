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

type AuthHeader string

const (
	AuthHeaderAuto       AuthHeader = "auto"
	AuthHeaderBearer     AuthHeader = "authorization"
	AuthHeaderAPIKey     AuthHeader = "x-api-key"
	AuthHeaderGoogAPIKey AuthHeader = "x-goog-api-key"
)

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
	// ResponseHeaderTimeout sets http.Transport.ResponseHeaderTimeout on the
	// internal HTTP client. Leave at 0 (the default) to match fetch behavior
	// and avoid aborting slow-starting streaming responses.
	//
	// This field has no effect when HTTPClient is supplied by the caller.
	ResponseHeaderTimeout time.Duration
	Timeout               time.Duration
	UserAgent             string
	HTTPClient            *http.Client
	Retry                 RetryConfig
	AuthHeader            AuthHeader
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

	if c.AuthHeader == "" {
		c.AuthHeader = AuthHeaderAuto
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
