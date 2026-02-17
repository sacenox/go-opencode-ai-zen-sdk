package zen

import (
	"context"
	"encoding/json"
)

func (c *Client) CreateModelContent(ctx context.Context, model string, body any, raw json.RawMessage) (json.RawMessage, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, err
	}
	data, _, err := c.doRequest(ctx, "POST", "/models/"+model, payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) CreateModelContentTyped(ctx context.Context, model string, req GeminiRequest) (json.RawMessage, error) {
	return c.CreateModelContent(ctx, model, req, nil)
}

func (c *Client) StreamModelContent(ctx context.Context, model string, body any, raw json.RawMessage) (<-chan StreamEvent, <-chan error, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, nil, err
	}

	stream, err := c.startStream(ctx, "POST", "/models/"+model, payload)
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
