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

func main() {
	apiKey := os.Getenv("OPENCODE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENCODE_API_KEY is required")
		os.Exit(1)
	}
	debugSSE := os.Getenv("DEBUG_SSE") == "1"

	client, err := zen.NewClient(zen.Config{APIKey: apiKey})
	if err != nil {
		panic(err)
	}

	tools := map[string]func(context.Context, json.RawMessage) (string, error){
		"add": func(_ context.Context, args json.RawMessage) (string, error) {
			var input struct {
				A float64 `json:"a"`
				B float64 `json:"b"`
			}
			if err := json.Unmarshal(args, &input); err != nil {
				return "", err
			}
			return fmt.Sprintf("%.0f", input.A+input.B), nil
		},
	}
	toolName := singleToolName(tools)

	models := resolveModels(os.Args[1:])
	for _, model := range models {
		fmt.Printf("=== Model: %s ===\n", model)
		if err := runAgentLoop(client, model, debugSSE, tools, toolName); err != nil {
			fmt.Fprintf(os.Stderr, "model %s failed: %v\n", model, err)
			if apiErr, ok := err.(*zen.APIError); ok && len(apiErr.Body) > 0 {
				fmt.Fprintf(os.Stderr, "api error body: %s\n", string(apiErr.Body))
			}
		}
	}
}

func runAgentLoop(client *zen.Client, model string, debugSSE bool, tools map[string]func(context.Context, json.RawMessage) (string, error), toolName string) error {
	messages := []zen.NormalizedMessage{{
		Role:    "user",
		Content: "What is 3 + 4, then double it?",
	}}

	maxSteps := 6
	for step := 0; step < maxSteps; step++ {
		req := zen.NormalizedRequest{
			Model:     model,
			System:    "You are a precise assistant. Always use the add tool for arithmetic, including doubling by adding a number to itself.",
			Messages:  messages,
			Reasoning: &zen.NormalizedReasoning{Effort: "low"},
			Tools: []zen.NormalizedTool{
				{
					Name:        "add",
					Description: "Adds two numbers",
					Parameters:  json.RawMessage(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}`),
				},
			},
			ToolChoice: &zen.NormalizedToolChoice{Type: zen.ToolChoiceAuto},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		deltas, errs, err := streamDeltas(ctx, client, req, debugSSE)
		if err != nil {
			cancel()
			return err
		}

		var text strings.Builder
		accumulator := zen.NewToolCallAccumulator()

		for d := range deltas {
			switch d.Type {
			case zen.DeltaText:
				text.WriteString(d.Content)
			case zen.DeltaReasoning:
				fmt.Printf("[reasoning] %s", d.Content)
			case zen.DeltaToolCallBegin, zen.DeltaToolCallArgumentsDelta, zen.DeltaToolCallDone:
				accumulator.Apply(d)
			}
		}
		if err := <-errs; err != nil {
			cancel()
			return err
		}
		cancel()

		calls := accumulator.CompleteCalls()
		if len(calls) == 0 {
			messages = append(messages, zen.NormalizedMessage{
				Role:    "assistant",
				Content: text.String(),
			})
			fmt.Printf("\n[assistant] %s\n", text.String())
			return nil
		}

		assistant := zen.NormalizedMessage{Role: "assistant"}
		for i := range calls {
			if calls[i].Name == "" && toolName != "" {
				calls[i].Name = toolName
			}
			assistant.ToolCalls = append(assistant.ToolCalls, zen.NormalizedToolCall{
				ID:        calls[i].ID,
				Name:      calls[i].Name,
				Arguments: calls[i].Arguments,
			})
		}
		messages = append(messages, assistant)

		for _, call := range calls {
			tool := tools[call.Name]
			result := ""
			if tool == nil {
				result = "unknown tool: " + call.Name
			} else {
				out, err := tool(context.Background(), call.Arguments)
				if err != nil {
					result = "error: " + err.Error()
				} else {
					result = out
				}
			}
			fmt.Printf("\n[tool:%s] %s\n", call.Name, result)
			messages = append(messages, zen.NormalizedMessage{
				Role:         "tool",
				ToolCallID:   call.ID,
				FunctionName: call.Name,
				Content:      result,
			})
		}
	}
	return fmt.Errorf("max steps reached without final response")
}

func streamDeltas(ctx context.Context, client *zen.Client, req zen.NormalizedRequest, debug bool) (<-chan zen.NormalizedDelta, <-chan error, error) {
	if !debug {
		return client.Stream(ctx, req)
	}

	evCh, errCh, err := client.StreamEvents(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan zen.NormalizedDelta)
	outErr := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(outErr)
		for ev := range evCh {
			if ev.Event != "" {
				fmt.Printf("[sse:%s] %s\n", ev.Event, ev.Raw)
			} else {
				fmt.Printf("[sse] %s\n", ev.Raw)
			}
			for _, delta := range zen.ParseNormalizedEvent(ev) {
				out <- delta
			}
		}
		if err := <-errCh; err != nil {
			outErr <- err
		}
	}()

	return out, outErr, nil
}

func resolveModels(args []string) []string {
	if len(args) > 0 {
		models := make([]string, 0, len(args))
		for _, arg := range args {
			if normalized := normalizeModelAlias(arg); normalized != "" {
				models = append(models, normalized)
			}
		}
		if len(models) > 0 {
			return models
		}
	}
	if raw := strings.TrimSpace(os.Getenv("MODELS")); raw != "" {
		parts := strings.Split(raw, ",")
		models := make([]string, 0, len(parts))
		for _, part := range parts {
			if v := strings.TrimSpace(part); v != "" {
				if normalized := normalizeModelAlias(v); normalized != "" {
					models = append(models, normalized)
				}
			}
		}
		if len(models) > 0 {
			return models
		}
	}
	return []string{"gpt-5.1"}
}

func singleToolName(tools map[string]func(context.Context, json.RawMessage) (string, error)) string {
	if len(tools) != 1 {
		return ""
	}
	for name := range tools {
		return name
	}
	return ""
}

func normalizeModelAlias(value string) string {
	model := strings.ToLower(strings.TrimSpace(value))
	if model == "" {
		return ""
	}
	if strings.HasPrefix(model, "opencode/") {
		model = strings.TrimPrefix(model, "opencode/")
	}
	model = strings.ReplaceAll(model, " ", "-")
	model = strings.ReplaceAll(model, "_", "-")

	switch model {
	case "haiku", "claude-haiku", "claude-haiku-3-5", "claude-3-5-haiku", "claude-haiku-4-5":
		return "claude-3-5-haiku"
	case "gemini-3-pro", "gemini-3-pro-preview", "gemini-3.0-pro", "gemini-3-pro-preview-raw":
		return "gemini-3-pro"
	case "minimax2.5", "minimax-2.5", "minimax-m2.5":
		return "minimax-m2.5"
	}

	return model
}
