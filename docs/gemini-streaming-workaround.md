# Gemini: Streaming Workaround for the Zen Proxy

## Background

The opencode zen proxy (`https://opencode.ai/zen/v1`) is a server-side gateway that
translates between the native wire formats of different AI providers and a unified
request/response model used by the opencode platform. Clients send requests to
provider-specific endpoint prefixes and the proxy forwards them to the correct
upstream API, handles authentication, tracks usage for billing, and converts
responses back.

The three main endpoint families are:

| Prefix              | Provider  | Auth header         |
|---------------------|-----------|---------------------|
| `/responses`        | OpenAI    | `Authorization: Bearer` |
| `/messages`         | Anthropic | `x-api-key`         |
| `/models/<model>`   | Google    | `x-goog-api-key`    |

Gemini models are accessed through the `/models/<model>` prefix. The proxy route
that handles this is a SolidStart dynamic segment (`[model].ts`) which reads the
model name from the URL and dispatches to the `googleHelper` provider adapter.

---

## The Bug

### Server-side: `json.usage` vs `json.usageMetadata`

The zen proxy's central request handler (`handler.ts`) has a single code path for
non-streaming responses:

```typescript
// handler.ts ~line 190
if (!isStream) {
  const json = await res.json()
  const usageInfo = providerInfo.normalizeUsage(json.usage)   // <-- BUG
  ...
}
```

`json.usage` is the field name used by OpenAI-style responses. Gemini's upstream
API, however, places token counts inside a different field called `usageMetadata`:

```json
{
  "candidates": [...],
  "usageMetadata": {
    "promptTokenCount": 11,
    "candidatesTokenCount": 3,
    "totalTokenCount": 14
  }
}
```

When the proxy calls `normalizeUsage(json.usage)` for a Gemini response,
`json.usage` is `undefined`. The `googleHelper.normalizeUsage` function then
immediately tries to read `usage.promptTokenCount`, which throws:

```
Cannot read properties of undefined (reading 'promptTokenCount')
```

The proxy returns this as an HTTP 500 to the client.

### Client-side: hardcoded `Authorization: Bearer` in streaming

A second, independent bug existed in this SDK's own `startStream` function
(`stream.go`). It hardcoded the OpenAI-style authorization header for all
requests, regardless of provider:

```go
// stream.go (before fix)
req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
```

The non-streaming `doRequest` function in `http.go` already had the correct
per-provider routing:

```go
switch {
case strings.HasPrefix(path, "/messages"):
    req.Header.Set("x-api-key", c.cfg.APIKey)
case strings.HasPrefix(path, "/models/") && strings.Contains(path, "gemini"):
    req.Header.Set("x-goog-api-key", c.cfg.APIKey)
default:
    req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
}
```

But `startStream` bypassed `doRequest` entirely and opened its own `http.Request`,
so this routing logic was never applied. Any streaming Gemini call would send
`Authorization: Bearer <key>` and be rejected by the proxy's authentication layer.

---

## Why the opencode CLI Was Not Affected

The opencode CLI uses `@ai-sdk/google` (the Vercel AI SDK's Google adapter) to
talk to Gemini. That adapter **always streams** — it sends requests to
`:streamGenerateContent?alt=sse` and processes the SSE response incrementally
using `streamText`. It never calls the non-streaming path in the proxy.

The proxy's streaming handler reads usage correctly:

```typescript
// google.ts — createUsageParser
parse: (chunk: string) => {
  ...
  json = JSON.parse(chunk.slice(6)) as { usageMetadata?: Usage }
  if (!json.usageMetadata) return
  usage = json.usageMetadata   // reads the right field
},
retrieve: () => usage,
```

And the streaming section of `handler.ts` guards against `usage` being absent:

```typescript
const usage = usageParser.retrieve()
if (usage) {                             // safe: guarded
  const usageInfo = providerInfo.normalizeUsage(usage)
  ...
}
```

So the CLI avoids the crash entirely by never taking the broken non-streaming code
path.

---

## URL Format: `:generateContent` and `:streamGenerateContent?alt=sse`

Google's Gemini API differentiates between the two generation modes via an
**action suffix** appended to the model path:

| Mode           | URL suffix                        |
|----------------|-----------------------------------|
| Non-streaming  | `:generateContent`                |
| Streaming SSE  | `:streamGenerateContent?alt=sse`  |

The zen proxy's model route handler parses this suffix to determine `isStream`:

```typescript
// [model].ts
parseModel: (url, body) => url.split("/").pop()?.split(":")?.[0] ?? "",
parseIsStream: (url, body) =>
  url.split("/").pop()?.split(":")?.[1]?.startsWith("streamGenerateContent") ?? false,
```

Without the suffix, `parseIsStream` returns `false` and the request takes the
non-streaming path — which is the broken one for Gemini.

This SDK initially called `/models/<model>` with no suffix at all. While the proxy
still routes the request correctly to `googleHelper`, it falls through to the
`!isStream` branch and crashes on `json.usage`.

---

## The Workaround

Since we cannot modify the server, both fixes are implemented client-side.

### Fix 1 — `stream.go`: per-provider auth header in `startStream`

`startStream` now mirrors the same switch that `doRequest` uses:

```go
switch {
case strings.HasPrefix(path, "/messages"):
    req.Header.Set("x-api-key", c.cfg.APIKey)
case strings.HasPrefix(path, "/models/") && strings.Contains(path, "gemini"):
    req.Header.Set("x-goog-api-key", c.cfg.APIKey)
default:
    req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
}
```

This ensures that streaming Gemini requests are authenticated correctly.

### Fix 2 — `gemini.go`: `CreateModelContent` uses streaming internally

`CreateModelContent` (the public non-streaming API) now calls
`:streamGenerateContent?alt=sse` internally, collects all SSE chunks from the
stream, and returns the last non-empty chunk as the response:

```go
func (c *Client) CreateModelContent(ctx context.Context, model string, body any, raw json.RawMessage) (json.RawMessage, error) {
    payload, err := jsonBody(body, raw)
    if err != nil {
        return nil, err
    }

    stream, err := c.startStream(ctx, "POST", "/models/"+model+":streamGenerateContent?alt=sse", payload)
    if err != nil {
        return nil, err
    }
    defer func() { _ = stream.Close() }()

    var last json.RawMessage
    for ev := range stream.Events {
        if len(ev.Data) > 0 {
            last = ev.Data
        }
    }
    if stream.Err != nil {
        return nil, stream.Err
    }
    if last == nil {
        return nil, fmt.Errorf("gemini: empty response")
    }
    return last, nil
}
```

The Gemini streaming SSE format sends one JSON object per chunk. Each chunk
represents a turn of candidate content. The final chunk always carries the full
`usageMetadata`. By returning the last chunk, callers get both the generated text
and accurate token counts.

`StreamModelContent` (the public streaming API) also uses the action suffix:

```go
stream, err := c.startStream(ctx, "POST", "/models/"+model+":streamGenerateContent?alt=sse", payload)
```

---

## Outcome

After both fixes, all Gemini models pass the integration test:

```
✓ gemini-3-pro    [models]  3.246s
✓ gemini-3-flash  [models]  1.351s
```

The approach matches exactly what `@ai-sdk/google` does: always stream, always
use the `:streamGenerateContent?alt=sse` action suffix, read usage from
`usageMetadata` in the final SSE chunk. The public `CreateModelContent` API
remains synchronous from the caller's perspective — the streaming is an
implementation detail.
