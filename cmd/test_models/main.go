package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	zen "github.com/sacenox/go-opencode-ai-zen-sdk"
)

// One representative model per endpoint type.
var probeModels = []string{"gpt-5.1", "claude-sonnet-4-6", "gemini-3-flash", "glm-5"}

type testResult struct {
	Model    string
	Endpoint string
	Success  bool
	Error    string
	Latency  time.Duration
	Request  string
	Response string
	Stream   bool
}

func main() {
	apiKey := os.Getenv("OPENCODE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENCODE_API_KEY environment variable is required")
		os.Exit(1)
	}

	client, err := zen.NewClient(zen.Config{APIKey: apiKey})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var results []testResult

	sections := []struct {
		name string
		fn   func(context.Context, *zen.Client, string) testResult
	}{
		{"Non-Streaming", testModel},
		{"Streaming", testModelStream},
		{"UnifiedCreate", testUnifiedCreate},
		{"UnifiedStream", testUnifiedStream},
		{"UnifiedCreateNormalized", testUnifiedCreateNormalized},
		{"UnifiedStreamNormalized", testUnifiedStreamNormalized},
	}

	for _, s := range sections {
		fmt.Printf("=== %s ===\n", s.name)
		for _, modelID := range probeModels {
			r := s.fn(ctx, client, modelID)
			results = append(results, r)
			printResult(r)
		}
		fmt.Println()
	}

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	fmt.Printf("=== Summary: %d/%d passed ===\n", successCount, len(results))
	for _, r := range results {
		if !r.Success {
			mode := "non-stream"
			if r.Stream {
				mode = "stream"
			}
			fmt.Printf("  FAILED [%s] %s (%s): %s\n", mode, r.Model, r.Endpoint, r.Error)
		}
	}
}

func testModel(ctx context.Context, client *zen.Client, modelID string) testResult {
	endpoint := routeForModel(modelID)
	start := time.Now()

	var resp json.RawMessage
	var err error
	var reqBody []byte

	switch endpoint {
	case "responses":
		body := map[string]any{"model": modelID, "input": "Say ok"}
		reqBody, _ = json.Marshal(body)
		resp, err = client.CreateResponse(ctx, body, nil)
	case "messages":
		body := map[string]any{
			"model":      modelID,
			"messages":   []map[string]string{{"role": "user", "content": "Say ok"}},
			"max_tokens": 64,
		}
		reqBody, _ = json.Marshal(body)
		resp, err = client.CreateMessage(ctx, body, nil)
	case "models":
		req := zen.GeminiRequest{
			Contents: []zen.GeminiContent{{Role: "user", Parts: []zen.GeminiPart{{Text: "Say ok"}}}},
		}
		reqBody, _ = json.Marshal(req)
		resp, err = client.CreateModelContentTyped(ctx, modelID, req)
	default:
		body := map[string]any{
			"model":    modelID,
			"messages": []map[string]string{{"role": "user", "content": "Say ok"}},
		}
		reqBody, _ = json.Marshal(body)
		resp, err = client.CreateChatCompletion(ctx, body, nil)
	}

	return makeResult(modelID, endpoint, false, string(reqBody), string(resp), err, time.Since(start))
}

func testModelStream(ctx context.Context, client *zen.Client, modelID string) testResult {
	endpoint := routeForModel(modelID)
	start := time.Now()

	var eventCh <-chan zen.StreamEvent
	var errCh <-chan error
	var err error
	var reqBody []byte

	switch endpoint {
	case "responses":
		body := map[string]any{"model": modelID, "input": "Say ok", "stream": true}
		reqBody, _ = json.Marshal(body)
		eventCh, errCh, err = client.StreamResponse(ctx, body, nil)
	case "messages":
		body := map[string]any{
			"model":      modelID,
			"messages":   []map[string]string{{"role": "user", "content": "Say ok"}},
			"max_tokens": 64,
			"stream":     true,
		}
		reqBody, _ = json.Marshal(body)
		eventCh, errCh, err = client.StreamMessage(ctx, body, nil)
	case "models":
		req := zen.GeminiRequest{
			Contents: []zen.GeminiContent{{Role: "user", Parts: []zen.GeminiPart{{Text: "Say ok"}}}},
		}
		reqBody, _ = json.Marshal(req)
		eventCh, errCh, err = client.StreamModelContentTyped(ctx, modelID, req)
	default:
		body := map[string]any{
			"model":    modelID,
			"messages": []map[string]string{{"role": "user", "content": "Say ok"}},
			"stream":   true,
		}
		reqBody, _ = json.Marshal(body)
		eventCh, errCh, err = client.StreamChatCompletion(ctx, body, nil)
	}

	return drainStream(modelID, endpoint, string(reqBody), eventCh, errCh, err, start)
}

func unifiedBody(modelID string) (any, string) {
	m := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.HasPrefix(m, "gpt-"):
		return map[string]any{"model": modelID, "input": "Say ok"}, "responses"
	case strings.HasPrefix(m, "claude-"):
		return map[string]any{
			"model":      modelID,
			"messages":   []map[string]string{{"role": "user", "content": "Say ok"}},
			"max_tokens": 64,
		}, "messages"
	case strings.HasPrefix(m, "gemini-"):
		return zen.GeminiRequest{
			Contents: []zen.GeminiContent{{Role: "user", Parts: []zen.GeminiPart{{Text: "Say ok"}}}},
		}, "models"
	default:
		return map[string]any{
			"model":    modelID,
			"messages": []map[string]string{{"role": "user", "content": "Say ok"}},
		}, "chat_completions"
	}
}

func testUnifiedCreate(ctx context.Context, client *zen.Client, modelID string) testResult {
	body, endpoint := unifiedBody(modelID)
	start := time.Now()
	reqBody, _ := json.Marshal(body)

	resp, err := client.UnifiedCreate(ctx, zen.UnifiedRequest{Model: modelID, Body: body})
	if err != nil {
		return makeResult(modelID, endpoint, false, string(reqBody), "", err, time.Since(start))
	}
	return makeResult(modelID, string(resp.Endpoint), false, string(reqBody), truncate(string(resp.Body), 100), nil, time.Since(start))
}

func testUnifiedStream(ctx context.Context, client *zen.Client, modelID string) testResult {
	body, endpoint := unifiedBody(modelID)
	start := time.Now()
	reqBody, _ := json.Marshal(body)

	eventCh, errCh, err := client.UnifiedStream(ctx, zen.UnifiedRequest{Model: modelID, Body: body, Stream: true})
	return drainUnifiedStream(modelID, endpoint, string(reqBody), eventCh, errCh, err, start)
}

func testUnifiedCreateNormalized(ctx context.Context, client *zen.Client, modelID string) testResult {
	req := zen.NormalizedRequest{
		Model:    modelID,
		Messages: []zen.NormalizedMessage{{Role: "user", Content: "Say ok"}},
	}
	start := time.Now()
	reqBody, _ := json.Marshal(req)

	resp, err := client.UnifiedCreateNormalized(ctx, req)
	if err != nil {
		return makeResult(modelID, "auto", false, string(reqBody), "", err, time.Since(start))
	}
	return makeResult(modelID, string(resp.Endpoint), false, string(reqBody), truncate(string(resp.Body), 100), nil, time.Since(start))
}

func testUnifiedStreamNormalized(ctx context.Context, client *zen.Client, modelID string) testResult {
	req := zen.NormalizedRequest{
		Model:    modelID,
		Messages: []zen.NormalizedMessage{{Role: "user", Content: "Say ok"}},
		Stream:   true,
	}
	start := time.Now()
	reqBody, _ := json.Marshal(req)

	eventCh, errCh, err := client.UnifiedStreamNormalized(ctx, req)
	return drainUnifiedStream(modelID, "auto", string(reqBody), eventCh, errCh, err, start)
}

// drainStream consumes a StreamEvent channel and builds a testResult.
func drainStream(modelID, endpoint, reqBody string, eventCh <-chan zen.StreamEvent, errCh <-chan error, initErr error, start time.Time) testResult {
	if initErr != nil {
		return makeResult(modelID, endpoint, true, reqBody, "", initErr, time.Since(start))
	}
	var count int
	var last string
	for ev := range eventCh {
		count++
		if len(ev.Data) > 0 {
			last = string(ev.Data)
		}
	}
	if err := <-errCh; err != nil {
		return makeResult(modelID, endpoint, true, reqBody, "", err, time.Since(start))
	}
	return makeResult(modelID, endpoint, true, reqBody, fmt.Sprintf("events=%d last=%s", count, truncate(last, 80)), nil, time.Since(start))
}

// drainUnifiedStream consumes a UnifiedEvent channel and builds a testResult.
func drainUnifiedStream(modelID, endpoint, reqBody string, eventCh <-chan zen.UnifiedEvent, errCh <-chan error, initErr error, start time.Time) testResult {
	if initErr != nil {
		return makeResult(modelID, endpoint, true, reqBody, "", initErr, time.Since(start))
	}
	var count int
	var last string
	var resolved string
	for ev := range eventCh {
		count++
		resolved = string(ev.Endpoint)
		if len(ev.Data) > 0 {
			last = string(ev.Data)
		}
	}
	if err := <-errCh; err != nil {
		return makeResult(modelID, endpoint, true, reqBody, "", err, time.Since(start))
	}
	if resolved != "" {
		endpoint = resolved
	}
	return makeResult(modelID, endpoint, true, reqBody, fmt.Sprintf("events=%d last=%s", count, truncate(last, 80)), nil, time.Since(start))
}

func makeResult(modelID, endpoint string, stream bool, req, resp string, err error, latency time.Duration) testResult {
	r := testResult{
		Model:    modelID,
		Endpoint: endpoint,
		Stream:   stream,
		Latency:  latency,
		Request:  req,
		Response: resp,
	}
	if err != nil {
		r.Error = err.Error()
	} else {
		r.Success = true
	}
	return r
}

func routeForModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(m, "gpt-"):
		return "responses"
	case strings.HasPrefix(m, "claude-"):
		return "messages"
	case strings.HasPrefix(m, "gemini-"):
		return "models"
	default:
		return "chat_completions"
	}
}

func printResult(r testResult) {
	status := "✓"
	if !r.Success {
		status = "✗"
	}
	mode := "non-stream"
	if r.Stream {
		mode = "stream"
	}
	fmt.Printf("  %s %-25s [%-15s] [%-10s] %v\n", status, r.Model, r.Endpoint, mode, r.Latency)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
