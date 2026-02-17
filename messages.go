package zen

import (
	"context"
	"encoding/json"
)

func (c *Client) CreateMessage(ctx context.Context, body any, raw json.RawMessage) (json.RawMessage, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, err
	}
	data, _, err := c.doRequest(ctx, "POST", "/messages", payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) CreateMessageTyped(ctx context.Context, req MessagesRequest) (json.RawMessage, error) {
	return c.CreateMessage(ctx, req, nil)
}

func (c *Client) StreamMessage(ctx context.Context, body any, raw json.RawMessage) (<-chan StreamEvent, <-chan error, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, nil, err
	}

	stream, err := c.startStream(ctx, "POST", "/messages", payload)
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

func (c *Client) StreamMessageTyped(ctx context.Context, req MessagesRequest) (<-chan StreamEvent, <-chan error, error) {
	return c.StreamMessage(ctx, req, nil)
}
