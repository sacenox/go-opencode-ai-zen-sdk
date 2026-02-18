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

type testResult struct {
	Model    string
	Endpoint string
	Success  bool
	Error    string
	Latency  time.Duration
	Request  string
	Response string
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

	fmt.Println("Fetching available models...")
	models, err := client.ListModels(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list models: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d models\n\n", len(models.Data))

	var results []testResult

	for _, model := range models.Data {
		result := testModel(ctx, client, model.ID)
		results = append(results, result)
		printResult(result)
	}

	fmt.Println("\n=== Summary ===")
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	fmt.Printf("Total: %d, Success: %d, Failed: %d\n", len(results), successCount, len(results)-successCount)

	for _, r := range results {
		if !r.Success {
			fmt.Printf("FAILED: %s (%s) - %s\n", r.Model, r.Endpoint, r.Error)
			fmt.Printf("  Request: %s\n", truncate(r.Request, 200))
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
		body := map[string]any{
			"model": modelID,
			"input": "Say ok",
		}
		reqBody, _ = json.Marshal(body)
		resp, err = client.CreateResponse(ctx, body, nil)
	case "messages":
		body := map[string]any{
			"model":      modelID,
			"messages":   []map[string]string{{"role": "user", "content": "Say ok"}},
			"max_tokens": 10,
		}
		reqBody, _ = json.Marshal(body)
		resp, err = client.CreateMessage(ctx, body, nil)
	case "models":
		req := zen.GeminiRequest{
			Contents: []zen.GeminiContent{
				{Role: "user", Parts: []zen.GeminiPart{{Text: "Say ok"}}},
			},
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

	latency := time.Since(start)

	result := testResult{
		Model:    modelID,
		Endpoint: endpoint,
		Latency:  latency,
		Request:  string(reqBody),
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Response = string(resp)
	return result
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
	fmt.Printf("%s %-30s [%-15s] %v\n", status, r.Model, r.Endpoint, r.Latency)
}

func intPtr(i int) *int {
	return &i
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
