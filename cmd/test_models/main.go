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
var probeModels = []string{"gpt-5.1", "claude-sonnet-4-6", "gemini-3-flash", "kimi-k2-thinking"}

type testResult struct {
	Model        string
	Endpoint     string
	Success      bool
	Error        string
	Latency      time.Duration
	Request      string
	Response     string
	Stream       bool
	InputTokens  int
	OutputTokens int
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

	sections := []struct {
		name string
		fn   func(context.Context, *zen.Client, string) testResult
	}{
		{"StreamEvents", testStreamEvents},
		{"StreamParsed", testStreamParsed},
		{"ToolHistory (stream)", testToolHistoryStream},
		{"Reasoning (stream)", testReasoningStream},
	}

	var results []testResult

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
			fmt.Printf("  FAILED [stream] %s (%s): %s\n", r.Model, r.Endpoint, r.Error)
		}
	}
}

func testStreamEvents(ctx context.Context, client *zen.Client, modelID string) testResult {
	start := time.Now()
	endpoint := routeForModel(modelID)
	req := zen.NormalizedRequest{
		Model:    modelID,
		Messages: []zen.NormalizedMessage{{Role: "user", Content: "Say ok"}},
	}
	reqBody, _ := json.Marshal(req)

	eventCh, errCh, err := client.StreamEvents(ctx, req)
	return drainUnifiedStream(modelID, endpoint, string(reqBody), eventCh, errCh, err, start)
}

func testStreamParsed(ctx context.Context, client *zen.Client, modelID string) testResult {
	start := time.Now()
	endpoint := routeForModel(modelID)
	req := zen.NormalizedRequest{
		Model:    modelID,
		Messages: []zen.NormalizedMessage{{Role: "user", Content: "Say ok"}},
	}
	reqBody, _ := json.Marshal(req)

	deltaCh, errCh, err := client.Stream(ctx, req)
	if err != nil {
		return makeResult(modelID, endpoint, true, string(reqBody), "", err, time.Since(start), 0, 0)
	}

	var textBuf, reasoningBuf strings.Builder
	var count, inTok, outTok int
	for d := range deltaCh {
		count++
		switch d.Type {
		case zen.DeltaText:
			textBuf.WriteString(d.Content)
		case zen.DeltaReasoning:
			reasoningBuf.WriteString(d.Content)
		case zen.DeltaUsage:
			if d.InputTokens > inTok {
				inTok = d.InputTokens
			}
			outTok += d.OutputTokens
		}
	}
	if streamErr := <-errCh; streamErr != nil {
		return makeResult(modelID, endpoint, true, string(reqBody), "", streamErr, time.Since(start), 0, 0)
	}

	resp := fmt.Sprintf("deltas=%d text=%s reasoning=%s", count, truncate(textBuf.String(), 40), truncate(reasoningBuf.String(), 40))
	return makeResult(modelID, endpoint, true, string(reqBody), resp, nil, time.Since(start), inTok, outTok)
}

// toolHistory returns a pre-built two-turn tool-use conversation:
//
//	user → assistant+tool_call → tool_result → user follow-up
func toolHistory() []zen.NormalizedMessage {
	return []zen.NormalizedMessage{
		{Role: "user", Content: "What is 3 + 4?"},
		{
			Role: "assistant",
			ToolCalls: []zen.NormalizedToolCall{
				{ID: "call_1", Name: "add", Arguments: json.RawMessage(`{"a":3,"b":4}`)},
			},
		},
		{Role: "tool", Content: "7", ToolCallID: "call_1"},
		{Role: "user", Content: "Thanks, what is the result doubled?"},
	}
}

func testToolHistoryStream(ctx context.Context, client *zen.Client, modelID string) testResult {
	start := time.Now()
	endpoint := routeForModel(modelID)
	req := zen.NormalizedRequest{
		Model:    modelID,
		System:   "You are a calculator assistant. When given a tool result, use it to answer the user.",
		Messages: toolHistory(),
		Tools: []zen.NormalizedTool{
			{
				Name:        "add",
				Description: "Adds two numbers",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}`),
			},
		},
	}
	reqBody, _ := json.Marshal(req)

	deltaCh, errCh, err := client.Stream(ctx, req)
	if err != nil {
		return makeResult(modelID, endpoint, true, string(reqBody), "", err, time.Since(start), 0, 0)
	}

	var textBuf strings.Builder
	var count, inTok, outTok int
	for d := range deltaCh {
		count++
		switch d.Type {
		case zen.DeltaText:
			textBuf.WriteString(d.Content)
		case zen.DeltaUsage:
			if d.InputTokens > inTok {
				inTok = d.InputTokens
			}
			outTok += d.OutputTokens
		}
	}
	if streamErr := <-errCh; streamErr != nil {
		return makeResult(modelID, endpoint, true, string(reqBody), "", streamErr, time.Since(start), 0, 0)
	}

	resp := fmt.Sprintf("deltas=%d text=%s", count, truncate(textBuf.String(), 60))
	return makeResult(modelID, endpoint, true, string(reqBody), resp, nil, time.Since(start), inTok, outTok)
}

func testReasoningStream(ctx context.Context, client *zen.Client, modelID string) testResult {
	start := time.Now()
	endpoint := routeForModel(modelID)
	req := zen.NormalizedRequest{
		Model:     modelID,
		Messages:  []zen.NormalizedMessage{{Role: "user", Content: "What is 2 + 2? Think step by step."}},
		Reasoning: &zen.NormalizedReasoning{Effort: "low"},
	}
	reqBody, _ := json.Marshal(req)

	deltaCh, errCh, err := client.Stream(ctx, req)
	if err != nil {
		return makeResult(modelID, endpoint, true, string(reqBody), "", err, time.Since(start), 0, 0)
	}

	var textBuf, reasoningBuf strings.Builder
	var deltaCount, inTok, outTok int
	for d := range deltaCh {
		deltaCount++
		switch d.Type {
		case zen.DeltaText:
			textBuf.WriteString(d.Content)
		case zen.DeltaReasoning:
			reasoningBuf.WriteString(d.Content)
		case zen.DeltaUsage:
			if d.InputTokens > inTok {
				inTok = d.InputTokens
			}
			outTok += d.OutputTokens
		}
	}
	if streamErr := <-errCh; streamErr != nil {
		return makeResult(modelID, endpoint, true, string(reqBody), "", streamErr, time.Since(start), 0, 0)
	}

	if reasoningBuf.Len() == 0 {
		return makeResult(modelID, endpoint, true, string(reqBody),
			fmt.Sprintf("deltas=%d text=%s", deltaCount, truncate(textBuf.String(), 60)),
			fmt.Errorf("model %s: stream contained no reasoning/thinking deltas", modelID),
			time.Since(start), inTok, outTok)
	}
	return makeResult(modelID, endpoint, true, string(reqBody),
		fmt.Sprintf("deltas=%d reasoning=%s text=%s",
			deltaCount,
			truncate(reasoningBuf.String(), 40),
			truncate(textBuf.String(), 40)),
		nil, time.Since(start), inTok, outTok)
}

// drainUnifiedStream consumes a UnifiedEvent channel and builds a testResult.
// StreamEvents operates at the raw event level and does not parse DeltaUsage,
// so token counts are not available here.
func drainUnifiedStream(modelID, endpoint, reqBody string, eventCh <-chan zen.UnifiedEvent, errCh <-chan error, initErr error, start time.Time) testResult {
	if initErr != nil {
		return makeResult(modelID, endpoint, true, reqBody, "", initErr, time.Since(start), 0, 0)
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
		return makeResult(modelID, endpoint, true, reqBody, "", err, time.Since(start), 0, 0)
	}
	if resolved != "" {
		endpoint = resolved
	}
	return makeResult(modelID, endpoint, true, reqBody, fmt.Sprintf("events=%d last=%s", count, truncate(last, 80)), nil, time.Since(start), 0, 0)
}

func makeResult(modelID, endpoint string, stream bool, req, resp string, err error, latency time.Duration, inTok, outTok int) testResult {
	r := testResult{
		Model:        modelID,
		Endpoint:     endpoint,
		Stream:       stream,
		Latency:      latency,
		Request:      req,
		Response:     resp,
		InputTokens:  inTok,
		OutputTokens: outTok,
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
	if strings.HasPrefix(m, "opencode/") {
		m = strings.TrimPrefix(m, "opencode/")
	}
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
	mode := "stream"
	usage := "-"
	if r.InputTokens > 0 || r.OutputTokens > 0 {
		usage = fmt.Sprintf("in=%d out=%d", r.InputTokens, r.OutputTokens)
	}
	fmt.Printf("  %s %-25s [%-15s] [%-10s] %-20s %v\n", status, r.Model, r.Endpoint, mode, usage, r.Latency)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
