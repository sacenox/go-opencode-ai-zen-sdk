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

type UnifiedRequest struct {
	Model    string
	Endpoint EndpointType
	Method   string
	Body     any
	Raw      json.RawMessage
	Stream   bool
}

type UnifiedResponse struct {
	Endpoint EndpointType
	Body     json.RawMessage
	Header   map[string][]string
}

type UnifiedEvent struct {
	Endpoint EndpointType
	Event    string
	Data     json.RawMessage
	Raw      string
}

func (c *Client) UnifiedCreate(ctx context.Context, req UnifiedRequest) (*UnifiedResponse, error) {
	if req.Stream {
		return nil, errors.New("zen: use UnifiedStream for streaming requests")
	}
	endpoint, path, method, err := c.resolveEndpoint(req)
	if err != nil {
		return nil, err
	}

	// Gemini model endpoints require the SSE path even for non-streaming requests;
	// delegate to CreateModelContent which handles the SSE collect-last-chunk pattern.
	if endpoint == EndpointModels && strings.TrimSpace(req.Model) != "" {
		data, err := c.CreateModelContent(ctx, req.Model, req.Body, req.Raw)
		if err != nil {
			return nil, err
		}
		return &UnifiedResponse{Endpoint: endpoint, Body: data}, nil
	}

	var payload []byte
	if req.Body != nil || req.Raw != nil || strings.ToUpper(method) != "GET" {
		var err error
		payload, err = jsonBody(req.Body, req.Raw)
		if err != nil {
			return nil, err
		}
	}

	data, header, err := c.doRequest(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}

	return &UnifiedResponse{
		Endpoint: endpoint,
		Body:     json.RawMessage(data),
		Header:   header,
	}, nil
}

func (c *Client) UnifiedStream(ctx context.Context, req UnifiedRequest) (<-chan UnifiedEvent, <-chan error, error) {
	if !req.Stream {
		return nil, nil, errors.New("zen: Stream must be true for UnifiedStream")
	}

	endpoint, path, method, err := c.resolveEndpoint(req)
	if err != nil {
		return nil, nil, err
	}

	// Gemini model endpoints require the SSE URL scheme; delegate to StreamModelContent.
	if endpoint == EndpointModels && strings.TrimSpace(req.Model) != "" {
		evCh, errCh, err := c.StreamModelContent(ctx, req.Model, req.Body, req.Raw)
		if err != nil {
			return nil, nil, err
		}
		events := make(chan UnifiedEvent)
		errs := make(chan error, 1)
		go func() {
			defer close(events)
			defer close(errs)
			for ev := range evCh {
				events <- UnifiedEvent{
					Endpoint: endpoint,
					Event:    ev.Event,
					Data:     ev.Data,
					Raw:      ev.Raw,
				}
			}
			if streamErr := <-errCh; streamErr != nil {
				errs <- streamErr
			}
		}()
		return events, errs, nil
	}

	var payload []byte
	if req.Body != nil || req.Raw != nil || strings.ToUpper(method) != "GET" {
		var err error
		payload, err = jsonBody(req.Body, req.Raw)
		if err != nil {
			return nil, nil, err
		}
	}

	events := make(chan UnifiedEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		stream, err := c.startStream(ctx, method, path, payload)
		if err != nil {
			errs <- err
			return
		}
		defer func() { _ = stream.Close() }()

		for ev := range stream.Events {
			events <- UnifiedEvent{
				Endpoint: endpoint,
				Event:    ev.Event,
				Data:     ev.Data,
				Raw:      ev.Raw,
			}
		}

		if stream.Err != nil {
			errs <- stream.Err
		}
	}()

	return events, errs, nil
}

func (c *Client) UnifiedCreateNormalized(ctx context.Context, req NormalizedRequest) (*UnifiedResponse, error) {
	if req.Stream {
		return nil, errors.New("zen: use UnifiedStreamNormalized for streaming requests")
	}

	endpoint := req.Endpoint
	if endpoint == EndpointAuto {
		endpoint = routeForModel(req.Model)
	}

	switch endpoint {
	case EndpointResponses:
		body, err := req.ToResponsesRequest()
		if err != nil {
			return nil, err
		}
		return c.UnifiedCreate(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body})
	case EndpointMessages:
		body, err := req.ToMessagesRequest()
		if err != nil {
			return nil, err
		}
		return c.UnifiedCreate(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body})
	case EndpointChatCompletions:
		body, err := req.ToChatCompletionsRequest()
		if err != nil {
			return nil, err
		}
		return c.UnifiedCreate(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body})
	case EndpointModels:
		body, err := req.ToGeminiRequest()
		if err != nil {
			return nil, err
		}
		return c.UnifiedCreate(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body})
	default:
		return nil, errors.New("zen: unsupported endpoint")
	}
}

func (c *Client) UnifiedStreamNormalized(ctx context.Context, req NormalizedRequest) (<-chan UnifiedEvent, <-chan error, error) {
	if !req.Stream {
		return nil, nil, errors.New("zen: Stream must be true for UnifiedStreamNormalized")
	}

	endpoint := req.Endpoint
	if endpoint == EndpointAuto {
		endpoint = routeForModel(req.Model)
	}

	switch endpoint {
	case EndpointResponses:
		body, err := req.ToResponsesRequest()
		if err != nil {
			return nil, nil, err
		}
		return c.UnifiedStream(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body, Stream: true})
	case EndpointMessages:
		body, err := req.ToMessagesRequest()
		if err != nil {
			return nil, nil, err
		}
		return c.UnifiedStream(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body, Stream: true})
	case EndpointChatCompletions:
		body, err := req.ToChatCompletionsRequest()
		if err != nil {
			return nil, nil, err
		}
		return c.UnifiedStream(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body, Stream: true})
	case EndpointModels:
		body, err := req.ToGeminiRequest()
		if err != nil {
			return nil, nil, err
		}
		return c.UnifiedStream(ctx, UnifiedRequest{Model: req.Model, Endpoint: endpoint, Body: body, Stream: true})
	default:
		return nil, nil, errors.New("zen: unsupported endpoint")
	}
}

func (c *Client) resolveEndpoint(req UnifiedRequest) (EndpointType, string, string, error) {
	endpoint := req.Endpoint
	if endpoint == EndpointAuto {
		endpoint = routeForModel(req.Model)
	}
	method := req.Method
	if method == "" {
		method = "POST"
	}

	switch endpoint {
	case EndpointResponses:
		return endpoint, "/responses", method, nil
	case EndpointMessages:
		return endpoint, "/messages", method, nil
	case EndpointChatCompletions:
		return endpoint, "/chat/completions", method, nil
	case EndpointModels:
		if strings.TrimSpace(req.Model) == "" {
			return endpoint, "/models", "GET", nil
		}
		return endpoint, "/models/" + req.Model, method, nil
	default:
		return endpoint, "", "", errors.New("zen: unsupported endpoint")
	}
}

func routeForModel(model string) EndpointType {
	m := strings.ToLower(strings.TrimSpace(model))
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
