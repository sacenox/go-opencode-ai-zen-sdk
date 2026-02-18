package zen

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateModelContent sends a request to the Gemini generateContent endpoint.
// It goes through the streaming path internally (:streamGenerateContent?alt=sse)
// to avoid a server-side bug where non-streaming responses have usage in
// usageMetadata rather than usage. The SSE chunks are collected and the last
// chunk (which contains the full response including usageMetadata) is returned.
func (c *Client) CreateModelContent(ctx context.Context, model string, body any, raw json.RawMessage) (json.RawMessage, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, err
	}

	stream, err := c.startStream(ctx, "POST", "/models/"+model+":streamGenerateContent?alt=sse", payload)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	// Collect all SSE chunks. Gemini sends one chunk per candidate turn;
	// the final chunk carries usageMetadata. Return the last non-empty chunk
	// as the canonical response.
	var last json.RawMessage
	for ev := range stream.Events {
		if len(ev.Data) > 0 {
			last = ev.Data
		}
	}
	if stream.Err != nil {
		return nil, stream.Err
	}
	if last == nil {
		return nil, fmt.Errorf("gemini: empty response")
	}
	return last, nil
}

func (c *Client) CreateModelContentTyped(ctx context.Context, model string, req GeminiRequest) (json.RawMessage, error) {
	return c.CreateModelContent(ctx, model, req, nil)
}

func (c *Client) StreamModelContent(ctx context.Context, model string, body any, raw json.RawMessage) (<-chan StreamEvent, <-chan error, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, nil, err
	}

	stream, err := c.startStream(ctx, "POST", "/models/"+model+":streamGenerateContent?alt=sse", payload)
	if err != nil {
		return nil, nil, err
	}

	errs := make(chan error, 1)
	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		defer close(errs)
		defer func() { _ = stream.Close() }()

		for ev := range stream.Events {
			out <- ev
		}
		if stream.Err != nil {
			errs <- stream.Err
		}
	}()

	return out, errs, nil
}

func (c *Client) StreamModelContentTyped(ctx context.Context, model string, req GeminiRequest) (<-chan StreamEvent, <-chan error, error) {
	return c.StreamModelContent(ctx, model, req, nil)
}
