package zen

import (
	"net/http"
	"strings"
)

func (c *Client) applyRequestHeaders(req *http.Request, endpoint EndpointType, streaming bool, forceAllAuth bool) {
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if streaming {
		req.Header.Set("Accept", "text/event-stream")
	}

	c.applyAuthHeaders(req, endpoint, forceAllAuth)
	if endpoint == EndpointMessages {
		req.Header.Set("anthropic-version", "2023-06-01")
		if streaming {
			req.Header.Set("anthropic-beta", "fine-grained-tool-streaming-2025-05-14")
		}
	}

	req.Header.Set("User-Agent", c.cfg.UserAgent)
}

func (c *Client) applyAuthHeaders(req *http.Request, endpoint EndpointType, forceAll bool) {
	if forceAll {
		c.setBearer(req)
		c.setAPIKey(req)
		c.setGoogAPIKey(req)
		return
	}

	switch c.cfg.AuthHeader {
	case AuthHeaderBearer:
		c.setBearer(req)
	case AuthHeaderAPIKey:
		c.setAPIKey(req)
	case AuthHeaderGoogAPIKey:
		c.setGoogAPIKey(req)
	default:
		switch endpoint {
		case EndpointMessages:
			c.setAPIKey(req)
		case EndpointModels:
			c.setGoogAPIKey(req)
		default:
			c.setBearer(req)
		}
	}
}

func (c *Client) setBearer(req *http.Request) {
	key := c.cfg.APIKey
	if !strings.HasPrefix(strings.ToLower(key), "bearer ") {
		key = "Bearer " + key
	}
	req.Header.Set("Authorization", key)
}

func (c *Client) setAPIKey(req *http.Request) {
	req.Header.Set("x-api-key", c.cfg.APIKey)
}

func (c *Client) setGoogAPIKey(req *http.Request) {
	req.Header.Set("x-goog-api-key", c.cfg.APIKey)
}
