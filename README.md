# go-opencode-ai-zen-sdk

Minimal Go SDK for OpenCode Zen.

## Install

```bash
go get github.com/sacenox/go-opencode-ai-zen-sdk
```

## Examples

See `examples/` for runnable examples (agent loop, tools, streaming).

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
- **Gemini tool calls:** preserve `ThoughtSignature` across tool call → tool result turns.
