# Normalized stream parsing

## What changed

A response parsing layer was added to complement the existing normalized request
layer. Two new public APIs are available:

**`ParseNormalizedEvent(ev UnifiedEvent) []NormalizedDelta`**
Parses a single raw `UnifiedEvent` into one or more `NormalizedDelta` values.
Use this if you are already consuming `UnifiedStreamNormalized` and want to
parse events yourself.

**`(c *Client) UnifiedStreamNormalizedParsed(...) (<-chan NormalizedDelta, ...)`**
All-in-one streaming API. Returns a channel of `NormalizedDelta` instead of raw
JSON events. This is the recommended way to consume streaming responses.

## Migration

Before this change, consumers of `UnifiedStreamNormalized` had to parse each
backend's SSE wire format manually to extract text and reasoning content:

```go
// Before — caller must know each endpoint's wire format
for ev := range eventCh {
    var chunk map[string]any
    json.Unmarshal(ev.Data, &chunk)
    // ... manually dig out content / reasoning_content / thinking_delta / thought parts
}
```

Switch to `UnifiedStreamNormalizedParsed` instead:

```go
// After
deltaCh, errCh, err := client.UnifiedStreamNormalizedParsed(ctx, req)
for d := range deltaCh {
    switch d.Type {
    case zen.DeltaText:
        fmt.Print(d.Content)
    case zen.DeltaReasoning:
        fmt.Print("[thinking] ", d.Content)
    case zen.DeltaToolCallBegin:
        fmt.Println("tool call:", d.ToolCallName)
    case zen.DeltaDone:
        // stream finished
    }
}
if err := <-errCh; err != nil {
    log.Fatal(err)
}
```

## NormalizedDelta fields

| Field | Set on |
|---|---|
| `Type` | always |
| `Content` | `DeltaText`, `DeltaReasoning` |
| `ToolCallIndex` | `DeltaToolCallBegin`, `DeltaToolCallArgumentsDelta`, `DeltaToolCallDone` |
| `ToolCallID` | `DeltaToolCallBegin`, `DeltaToolCallDone` |
| `ToolCallName` | `DeltaToolCallBegin`, `DeltaToolCallDone` |
| `ArgumentsDelta` | `DeltaToolCallArgumentsDelta` |
| `ArgumentsFull` | `DeltaToolCallDone` |

## Reasoning field mapping per endpoint

| Endpoint | Models | Wire field |
|---|---|---|
| `chat_completions` | Kimi, DeepSeek, etc. | `choices[0].delta.reasoning_content` |
| `responses` | `gpt-*` | `response.reasoning_summary_text.delta`, `response.reasoning.delta` |
| `messages` | `claude-*` | `content_block_delta` with `delta.type == "thinking_delta"` |
| `models` | `gemini-*` | `candidates[0].content.parts[].thought == true` |
