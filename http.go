package zen

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

var retryableStatus = map[int]bool{
	http.StatusTooManyRequests:     true,
	http.StatusInternalServerError: true,
	http.StatusBadGateway:          true,
	http.StatusServiceUnavailable:  true,
	http.StatusGatewayTimeout:      true,
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte, endpoint EndpointType, forceAllAuth bool) ([]byte, http.Header, error) {
	url := joinURL(c.cfg.BaseURL, path)
	if body == nil {
		body = []byte{}
	}

	retries := c.cfg.Retry.MaxRetries
	if !isIdempotent(method) && !c.cfg.Retry.RetryOnNonIdempotent {
		retries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, nil, err
		}

		c.applyRequestHeaders(req, endpoint, false, forceAllAuth)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < retries {
				time.Sleep(c.cfg.Retry.Backoff(attempt))
				continue
			}
			return nil, nil, err
		}

		payload, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.Header, readErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return payload, resp.Header, nil
		}

		apiErr := newAPIError(resp.StatusCode, resp.Header, payload)
		lastErr = apiErr
		if attempt < retries && retryableStatus[resp.StatusCode] {
			time.Sleep(c.cfg.Retry.Backoff(attempt))
			continue
		}
		return nil, resp.Header, apiErr
	}

	return nil, nil, lastErr
}

func jsonBody(v any, raw json.RawMessage) ([]byte, error) {
	if raw != nil {
		return raw, nil
	}
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}

func joinURL(base, path string) string {
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func isIdempotent(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
