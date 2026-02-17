package zen

import (
	"context"
	"encoding/json"
)

func (c *Client) CreateChatCompletion(ctx context.Context, body any, raw json.RawMessage) (json.RawMessage, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, err
	}
	data, _, err := c.doRequest(ctx, "POST", "/chat/completions", payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) CreateChatCompletionTyped(ctx context.Context, req ChatCompletionsRequest) (json.RawMessage, error) {
	return c.CreateChatCompletion(ctx, req, nil)
}

func (c *Client) StreamChatCompletion(ctx context.Context, body any, raw json.RawMessage) (<-chan StreamEvent, <-chan error, error) {
	payload, err := jsonBody(body, raw)
	if err != nil {
		return nil, nil, err
	}

	stream, err := c.startStream(ctx, "POST", "/chat/completions", payload)
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

func (c *Client) StreamChatCompletionTyped(ctx context.Context, req ChatCompletionsRequest) (<-chan StreamEvent, <-chan error, error) {
	return c.StreamChatCompletion(ctx, req, nil)
}
