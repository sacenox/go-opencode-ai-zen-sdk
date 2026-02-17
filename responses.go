package zen

import (
	"context"
	"encoding/json"
)

func (c *Client) CreateResponse(ctx context.Context, body any, raw json.RawMessage) (json.RawMessage, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, err
	}
	data, _, err := c.doRequest(ctx, "POST", "/responses", payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) CreateResponseTyped(ctx context.Context, req ResponsesRequest) (json.RawMessage, error) {
	return c.CreateResponse(ctx, req, nil)
}

func (c *Client) StreamResponse(ctx context.Context, body any, raw json.RawMessage) (<-chan StreamEvent, <-chan error, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, nil, err
	}

	stream, err := c.startStream(ctx, "POST", "/responses", payload)
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

func (c *Client) StreamResponseTyped(ctx context.Context, req ResponsesRequest) (<-chan StreamEvent, <-chan error, error) {
	return c.StreamResponse(ctx, req, nil)
}
