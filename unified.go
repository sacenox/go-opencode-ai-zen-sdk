package zen

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type EndpointType string

const (
	EndpointAuto            EndpointType = ""
	EndpointResponses       EndpointType = "responses"
	EndpointMessages        EndpointType = "messages"
	EndpointChatCompletions EndpointType = "chat_completions"
	EndpointModels          EndpointType = "models"
)

type UnifiedEvent struct {
	Endpoint EndpointType
	Event    string
	Data     json.RawMessage
	Raw      string
}

// StreamEvents is the unified streaming API. It routes the request based on
// the normalized model id and returns raw SSE events with the resolved endpoint.
func (c *Client) StreamEvents(ctx context.Context, req NormalizedRequest) (<-chan UnifiedEvent, <-chan error, error) {
	req.Model = stripOpencodePrefix(req.Model)
	endpoint, path, err := resolveStreamEndpoint(req)
	if err != nil {
		return nil, nil, err
	}

	req.Stream = true

	var body any
	switch endpoint {
	case EndpointResponses:
		payload, err := req.ToResponsesRequest()
		if err != nil {
			return nil, nil, err
		}
		body = payload
	case EndpointMessages:
		payload, err := req.ToMessagesRequest()
		if err != nil {
			return nil, nil, err
		}
		body = payload
	case EndpointChatCompletions:
		payload, err := req.ToChatCompletionsRequest()
		if err != nil {
			return nil, nil, err
		}
		body = payload
	case EndpointModels:
		payload, err := req.ToGeminiRequest()
		if err != nil {
			return nil, nil, err
		}
		body = payload
	default:
		return nil, nil, errors.New("zen: unsupported endpoint")
	}

	payload, err := jsonBody(body, nil)
	if err != nil {
		return nil, nil, err
	}

	stream, err := c.startStream(ctx, endpoint, "POST", path, payload)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan UnifiedEvent)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)
		defer func() { _ = stream.Close() }()

		for ev := range stream.Events {
			out <- UnifiedEvent{
				Endpoint: endpoint,
				Event:    ev.Event,
				Data:     ev.Data,
				Raw:      ev.Raw,
			}
		}
		if stream.Err != nil {
			errCh <- stream.Err
		}
	}()

	return out, errCh, nil
}

// Stream parses unified SSE events into normalized deltas.
func (c *Client) Stream(ctx context.Context, req NormalizedRequest) (<-chan NormalizedDelta, <-chan error, error) {
	evCh, errCh, err := c.StreamEvents(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan NormalizedDelta)
	outErr := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(outErr)
		for ev := range evCh {
			for _, delta := range ParseNormalizedEvent(ev) {
				out <- delta
			}
		}
		if streamErr := <-errCh; streamErr != nil {
			outErr <- streamErr
		}
	}()

	return out, outErr, nil
}

func resolveStreamEndpoint(req NormalizedRequest) (EndpointType, string, error) {
	endpoint := req.Endpoint
	if endpoint == EndpointAuto {
		endpoint = routeForModel(req.Model)
	}

	switch endpoint {
	case EndpointResponses:
		return endpoint, "/responses", nil
	case EndpointMessages:
		return endpoint, "/messages", nil
	case EndpointChatCompletions:
		return endpoint, "/chat/completions", nil
	case EndpointModels:
		model := strings.TrimSpace(stripOpencodePrefix(req.Model))
		if model == "" {
			return endpoint, "", errors.New("zen: model is required for model streaming")
		}
		return endpoint, "/models/" + model + ":streamGenerateContent?alt=sse", nil
	default:
		return endpoint, "", errors.New("zen: unsupported endpoint")
	}
}

func routeForModel(model string) EndpointType {
	m := normalizeModelID(model)
	switch {
	case strings.HasPrefix(m, "gpt-"):
		return EndpointResponses
	case strings.HasPrefix(m, "claude-"):
		return EndpointMessages
	case strings.HasPrefix(m, "gemini-"):
		return EndpointModels
	default:
		return EndpointChatCompletions
	}
}

func normalizeModelID(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "opencode/") {
		m = strings.TrimPrefix(m, "opencode/")
	}
	return m
}

func stripOpencodePrefix(model string) string {
	m := strings.TrimSpace(model)
	if strings.HasPrefix(strings.ToLower(m), "opencode/") {
		return m[len("opencode/"):]
	}
	return m
}
