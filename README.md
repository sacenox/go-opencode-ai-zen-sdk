# go-opencode-ai-zen-sdk

Minimal Go SDK for OpenCode Zen.

## Install

```bash
go get github.com/sacenox/go-opencode-ai-zen-sdk
```

## Agent Loop Example (tools + streaming)

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	zen "github.com/sacenox/go-opencode-ai-zen-sdk"
)

type toolCall struct {
	ID   string
	Name string
	Args strings.Builder
	Full string
}

func main() {
	client, err := zen.NewClient(zen.Config{APIKey: "your-key"})
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

	messages := []zen.NormalizedMessage{{
		Role:    "user",
		Content: "What is 3 + 4, then double it?",
	}}

	maxSteps := 6
	for step := 0; step < maxSteps; step++ {
		req := zen.NormalizedRequest{
			Model:     "gpt-5.1",
			System:    "You are a precise assistant. Use tools when needed.",
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
		deltas, errs, err := client.Stream(ctx, req)
		if err != nil {
			cancel()
			panic(err)
		}

		var text strings.Builder
		toolCalls := map[int]*toolCall{}
		var ordered []*toolCall

		for d := range deltas {
			switch d.Type {
			case zen.DeltaText:
				text.WriteString(d.Content)
			case zen.DeltaToolCallBegin:
				call := &toolCall{ID: d.ToolCallID, Name: d.ToolCallName}
				toolCalls[d.ToolCallIndex] = call
				ordered = append(ordered, call)
			case zen.DeltaToolCallArgumentsDelta:
				if call := toolCalls[d.ToolCallIndex]; call != nil {
					call.Args.WriteString(d.ArgumentsDelta)
				}
			case zen.DeltaToolCallDone:
				call := toolCalls[d.ToolCallIndex]
				if call == nil {
					call = &toolCall{}
					toolCalls[d.ToolCallIndex] = call
					ordered = append(ordered, call)
				}
				call.ID = d.ToolCallID
				call.Name = d.ToolCallName
				if d.ArgumentsFull != "" {
					call.Full = d.ArgumentsFull
				} else {
					call.Full = call.Args.String()
				}
			}
		}
		if err := <-errs; err != nil {
			cancel()
			panic(err)
		}
		cancel()

		if len(ordered) == 0 {
			messages = append(messages, zen.NormalizedMessage{
				Role:    "assistant",
				Content: text.String(),
			})
			fmt.Println(text.String())
			break
		}

		assistant := zen.NormalizedMessage{Role: "assistant"}
		for _, call := range ordered {
			assistant.ToolCalls = append(assistant.ToolCalls, zen.NormalizedToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: json.RawMessage(call.Full),
			})
		}
		messages = append(messages, assistant)

		for _, call := range ordered {
			tool := tools[call.Name]
			if tool == nil {
				messages = append(messages, zen.NormalizedMessage{
					Role:       "tool",
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("unknown tool: %s", call.Name),
				})
				continue
			}
			result, err := tool(context.Background(), json.RawMessage(call.Full))
			if err != nil {
				result = "error: " + err.Error()
			}
			messages = append(messages, zen.NormalizedMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}
}
```

## List Models

```go
resp, err := client.ListModels(context.Background())
if err != nil {
	panic(err)
}
fmt.Println(len(resp.Data))
```

## Development

```bash
gofmt -w *.go
go test ./...
golangci-lint run
```

## Disclaimer

This project is not affiliated with opencode.ai.

## Notes

- Base URL defaults to `https://opencode.ai/zen/v1`.
- Unified routing uses model prefixes:
  - `gpt-*` -> `/responses`
  - `claude-*` -> `/messages`
  - `gemini-*` -> `/models/<model>`
  - fallback -> `/chat/completions`
- **Streaming and timeouts:** `Config.Timeout` defaults to `0` (no timeout). Do not
  set it for streaming calls — `http.Client.Timeout` is a total round-trip deadline
  that fires while the SSE body is still being read, causing spurious
  `context deadline exceeded (Client.Timeout ...)` errors. Use the request
  `context.Context` to enforce deadlines instead.
