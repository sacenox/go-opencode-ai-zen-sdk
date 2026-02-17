# go-opencode-ai-zen-sdk

Minimal Go SDK for OpenCode Zen.

## Install

```bash
go get github.com/sacenox/go-opencode-ai-zen-sdk
```

## Usage

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	zen "github.com/sacenox/go-opencode-ai-zen-sdk"
)

func main() {
	client, err := zen.NewClient(zen.Config{APIKey: "your-key"})
	if err != nil {
		panic(err)
	}

	req := zen.UnifiedRequest{
		Model:  "gpt-5.2-codex",
		Body: map[string]any{
			"input": "Write a Go hello world",
		},
	}

	resp, err := client.UnifiedCreate(context.Background(), req)
	if err != nil {
		panic(err)
	}

	var payload map[string]any
	_ = json.Unmarshal(resp.Body, &payload)
	fmt.Println(payload)
}
```

## Normalized Request (reasoning + tools)

```go
req := zen.NormalizedRequest{
	Model:  "gpt-5.2-codex",
	System: "You are a precise coding assistant.",
	Messages: []zen.NormalizedMessage{
		{Role: "user", Content: "Explain what a mutex is."},
	},
	Reasoning: &zen.NormalizedReasoning{Effort: "low"},
	Tools: []zen.NormalizedTool{
		{
			Name:        "lookup",
			Description: "Query internal docs",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
		},
	},
	ToolChoice: &zen.NormalizedToolChoice{Type: zen.ToolChoiceAuto},
}

resp, err := client.UnifiedCreateNormalized(ctx, req)
if err != nil {
	panic(err)
}
fmt.Println(string(resp.Body))
```

## Gemini (models endpoint)

```go
req := zen.GeminiRequest{
	Contents: []zen.GeminiContent{
		{Role: "user", Parts: []zen.GeminiPart{{Text: "Explain Go interfaces"}}},
	},
}

resp, err := client.CreateModelContentTyped(ctx, "gemini-3-pro", req)
if err != nil {
	panic(err)
}
fmt.Println(string(resp))
```

## Streaming

```go
events, errs, err := client.UnifiedStream(ctx, zen.UnifiedRequest{
	Model:  "gpt-5.2-codex",
	Stream: true,
	Body: map[string]any{
		"input": "Stream me a poem",
		"stream": true,
	},
})
if err != nil {
	panic(err)
}

for ev := range events {
	fmt.Println(ev.Event, string(ev.Data))
}

if err := <-errs; err != nil {
	panic(err)
}
```

## List Models

```go
resp, err := client.ListModels(ctx)
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
