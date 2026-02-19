package zen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testCtx(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

// helper: build a UnifiedEvent with the given endpoint and JSON body.
func makeEvent(endpoint EndpointType, data string) UnifiedEvent {
	return UnifiedEvent{Endpoint: endpoint, Data: json.RawMessage(data)}
}

// helper: build a UnifiedEvent that also carries an SSE event name.
func makeEventNamed(endpoint EndpointType, event, data string) UnifiedEvent {
	return UnifiedEvent{Endpoint: endpoint, Event: event, Data: json.RawMessage(data)}
}

// newSSETestServer creates a test server that serves the given SSE body for all
// requests.
func newSSETestServer(t *testing.T, sse string) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	c, err := NewClient(Config{APIKey: "key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return server, c
}

func assertDeltaSequence(t *testing.T, got []NormalizedDelta, want ...NormalizedDeltaType) {
	t.Helper()
	if len(got) != len(want) {
		types := make([]NormalizedDeltaType, len(got))
		for i, d := range got {
			types[i] = d.Type
		}
		t.Fatalf("delta count mismatch: want %v, got %v", want, types)
	}
	for i, w := range want {
		if got[i].Type != w {
			t.Fatalf("delta[%d]: want %s, got %s", i, w, got[i].Type)
		}
	}
}

// ---------------------------------------------------------------------------
// chat/completions
// ---------------------------------------------------------------------------

func TestParseChatCompletionsText(t *testing.T) {
	ev := makeEvent(EndpointChatCompletions, `{"choices":[{"delta":{"content":"Hello"}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaText || deltas[0].Content != "Hello" {
		t.Fatalf("expected text delta 'Hello', got %+v", deltas)
	}
}

func TestParseChatCompletionsReasoning(t *testing.T) {
	ev := makeEvent(EndpointChatCompletions, `{"choices":[{"delta":{"reasoning_content":"thinking..."}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaReasoning || deltas[0].Content != "thinking..." {
		t.Fatalf("expected reasoning delta, got %+v", deltas)
	}
}

func TestParseChatCompletionsReasoningThenText(t *testing.T) {
	ev := makeEvent(EndpointChatCompletions, `{"choices":[{"delta":{"reasoning_content":"think","content":"answer"}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d: %+v", len(deltas), deltas)
	}
	if deltas[0].Type != DeltaReasoning {
		t.Fatalf("expected first delta to be reasoning, got %s", deltas[0].Type)
	}
	if deltas[1].Type != DeltaText {
		t.Fatalf("expected second delta to be text, got %s", deltas[1].Type)
	}
}

func TestParseChatCompletionsToolCall(t *testing.T) {
	ev := makeEvent(EndpointChatCompletions, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaToolCallBegin {
		t.Fatalf("expected tool_call_begin, got %+v", deltas)
	}
	if deltas[0].ToolCallID != "call_1" || deltas[0].ToolCallName != "get_weather" {
		t.Fatalf("tool call fields wrong: %+v", deltas[0])
	}
}

func TestParseChatCompletionsToolCallArguments(t *testing.T) {
	ev := makeEvent(EndpointChatCompletions, `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaToolCallArgumentsDelta {
		t.Fatalf("expected tool_call_arguments_delta, got %+v", deltas)
	}
	if deltas[0].ArgumentsDelta != `{"city":` {
		t.Fatalf("arguments delta wrong: %q", deltas[0].ArgumentsDelta)
	}
}

func TestParseChatCompletionsDone(t *testing.T) {
	ev := makeEvent(EndpointChatCompletions, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaDone {
		t.Fatalf("expected done delta, got %+v", deltas)
	}
}

// ---------------------------------------------------------------------------
// responses (OpenAI Responses API)
// ---------------------------------------------------------------------------

func TestParseResponsesText(t *testing.T) {
	ev := makeEventNamed(EndpointResponses, "response.output_text.delta", `{"type":"response.output_text.delta","delta":"Hi"}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaText || deltas[0].Content != "Hi" {
		t.Fatalf("expected text delta 'Hi', got %+v", deltas)
	}
}

func TestParseResponsesReasoningSummary(t *testing.T) {
	ev := makeEventNamed(EndpointResponses, "response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","delta":"reasoning..."}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaReasoning || deltas[0].Content != "reasoning..." {
		t.Fatalf("expected reasoning delta, got %+v", deltas)
	}
}

func TestParseResponsesReasoningDelta(t *testing.T) {
	// Also accept the older response.reasoning.delta event name.
	ev := makeEventNamed(EndpointResponses, "response.reasoning.delta", `{"type":"response.reasoning.delta","delta":"thought"}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaReasoning || deltas[0].Content != "thought" {
		t.Fatalf("expected reasoning delta, got %+v", deltas)
	}
}

func TestParseResponsesToolCallBegin(t *testing.T) {
	ev := makeEvent(EndpointResponses, `{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_abc","name":"search"}}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaToolCallBegin {
		t.Fatalf("expected tool_call_begin, got %+v", deltas)
	}
	if deltas[0].ToolCallID != "call_abc" || deltas[0].ToolCallName != "search" {
		t.Fatalf("tool call fields wrong: %+v", deltas[0])
	}
}

func TestParseResponsesToolCallArgumentsDelta(t *testing.T) {
	ev := makeEvent(EndpointResponses, `{"type":"response.function_call_arguments_delta","output_index":1,"delta":"{\"q\":"}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaToolCallArgumentsDelta {
		t.Fatalf("expected tool_call_arguments_delta, got %+v", deltas)
	}
	if deltas[0].ArgumentsDelta != `{"q":` {
		t.Fatalf("arguments delta wrong: %q", deltas[0].ArgumentsDelta)
	}
	if deltas[0].ToolCallIndex != 1 {
		t.Fatalf("tool call index wrong: %d", deltas[0].ToolCallIndex)
	}
}

func TestParseResponsesDone(t *testing.T) {
	ev := makeEvent(EndpointResponses, `{"type":"response.completed"}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaDone {
		t.Fatalf("expected done delta, got %+v", deltas)
	}
}

// ---------------------------------------------------------------------------
// messages (Anthropic)
// ---------------------------------------------------------------------------

func TestParseMessagesText(t *testing.T) {
	ev := makeEventNamed(EndpointMessages, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaText || deltas[0].Content != "Hello" {
		t.Fatalf("expected text delta, got %+v", deltas)
	}
}

func TestParseMessagesThinkingDelta(t *testing.T) {
	ev := makeEventNamed(EndpointMessages, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"I need to think..."}}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaReasoning || deltas[0].Content != "I need to think..." {
		t.Fatalf("expected reasoning delta, got %+v", deltas)
	}
}

func TestParseMessagesToolUseStart(t *testing.T) {
	ev := makeEventNamed(EndpointMessages, "content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_abc","name":"calculator"}}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaToolCallBegin {
		t.Fatalf("expected tool_call_begin, got %+v", deltas)
	}
	if deltas[0].ToolCallID != "toolu_abc" || deltas[0].ToolCallName != "calculator" {
		t.Fatalf("tool call fields wrong: %+v", deltas[0])
	}
	if deltas[0].ToolCallIndex != 1 {
		t.Fatalf("tool call index wrong: %d", deltas[0].ToolCallIndex)
	}
}

func TestParseMessagesToolInputDelta(t *testing.T) {
	ev := makeEventNamed(EndpointMessages, "content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"x\":"}}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaToolCallArgumentsDelta {
		t.Fatalf("expected tool_call_arguments_delta, got %+v", deltas)
	}
	if deltas[0].ArgumentsDelta != `{"x":` {
		t.Fatalf("arguments delta wrong: %q", deltas[0].ArgumentsDelta)
	}
}

func TestParseMessagesStop(t *testing.T) {
	ev := makeEventNamed(EndpointMessages, "message_stop", `{"type":"message_stop"}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaDone {
		t.Fatalf("expected done delta, got %+v", deltas)
	}
}

// ---------------------------------------------------------------------------
// models (Gemini)
// ---------------------------------------------------------------------------

func TestParseGeminiText(t *testing.T) {
	ev := makeEvent(EndpointModels, `{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaText || deltas[0].Content != "Hello" {
		t.Fatalf("expected text delta 'Hello', got %+v", deltas)
	}
}

func TestParseGeminiThought(t *testing.T) {
	ev := makeEvent(EndpointModels, `{"candidates":[{"content":{"parts":[{"text":"thinking...","thought":true}]}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 1 || deltas[0].Type != DeltaReasoning || deltas[0].Content != "thinking..." {
		t.Fatalf("expected reasoning delta, got %+v", deltas)
	}
}

func TestParseGeminiThoughtThenText(t *testing.T) {
	ev := makeEvent(EndpointModels, `{"candidates":[{"content":{"parts":[{"text":"I'm thinking","thought":true},{"text":"Here is the answer"}]}}]}`)
	deltas := ParseNormalizedEvent(ev)
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d: %+v", len(deltas), deltas)
	}
	if deltas[0].Type != DeltaReasoning {
		t.Fatalf("expected first delta reasoning, got %s", deltas[0].Type)
	}
	if deltas[1].Type != DeltaText {
		t.Fatalf("expected second delta text, got %s", deltas[1].Type)
	}
}

func TestParseGeminiFunctionCall(t *testing.T) {
	ev := makeEvent(EndpointModels, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"city":"Paris"}}}]}}]}`)
	deltas := ParseNormalizedEvent(ev)
	// Expect DeltaToolCallBegin + DeltaToolCallArgumentsDelta
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d: %+v", len(deltas), deltas)
	}
	if deltas[0].Type != DeltaToolCallBegin || deltas[0].ToolCallName != "get_weather" {
		t.Fatalf("expected tool_call_begin with name get_weather, got %+v", deltas[0])
	}
	if deltas[1].Type != DeltaToolCallArgumentsDelta {
		t.Fatalf("expected tool_call_arguments_delta, got %+v", deltas[1])
	}
}

func TestParseGeminiDone(t *testing.T) {
	ev := makeEvent(EndpointModels, `{"candidates":[{"content":{"parts":[{"text":"done"}]},"finishReason":"STOP"}]}`)
	deltas := ParseNormalizedEvent(ev)
	// text delta + done delta
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas (text+done), got %d: %+v", len(deltas), deltas)
	}
	if deltas[len(deltas)-1].Type != DeltaDone {
		t.Fatalf("last delta should be done, got %s", deltas[len(deltas)-1].Type)
	}
}

// ---------------------------------------------------------------------------
// Integration: UnifiedStreamNormalizedParsed end-to-end with a mock server
// ---------------------------------------------------------------------------

func TestUnifiedStreamNormalizedParsedChatCompletions(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"

	server, client := newSSETestServer(t, sse)
	defer server.Close()

	req := NormalizedRequest{
		Model:    "kimi-k2",
		Messages: []NormalizedMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	}
	deltaCh, errCh, err := client.UnifiedStreamNormalizedParsed(testCtx(t), req)
	if err != nil {
		t.Fatalf("UnifiedStreamNormalizedParsed: %v", err)
	}

	var deltas []NormalizedDelta
	for d := range deltaCh {
		deltas = append(deltas, d)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("stream error: %v", err)
	}

	assertDeltaSequence(t, deltas, DeltaReasoning, DeltaText, DeltaDone)
	if deltas[0].Content != "thinking" {
		t.Fatalf("reasoning content: want 'thinking', got %q", deltas[0].Content)
	}
	if deltas[1].Content != "answer" {
		t.Fatalf("text content: want 'answer', got %q", deltas[1].Content)
	}
}

func TestUnifiedStreamNormalizedParsedMessages(t *testing.T) {
	sse := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"hmm\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

	server, client := newSSETestServer(t, sse)
	defer server.Close()

	req := NormalizedRequest{
		Model:    "claude-sonnet-4-6",
		Messages: []NormalizedMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	}
	deltaCh, errCh, err := client.UnifiedStreamNormalizedParsed(testCtx(t), req)
	if err != nil {
		t.Fatalf("UnifiedStreamNormalizedParsed: %v", err)
	}

	var deltas []NormalizedDelta
	for d := range deltaCh {
		deltas = append(deltas, d)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("stream error: %v", err)
	}

	assertDeltaSequence(t, deltas, DeltaReasoning, DeltaText, DeltaDone)
}

func TestUnifiedStreamNormalizedParsedResponses(t *testing.T) {
	sse := "event: response.reasoning_summary_text.delta\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"reasoning\"}\n\n" +
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"text\"}\n\n" +
		"event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n"

	server, client := newSSETestServer(t, sse)
	defer server.Close()

	req := NormalizedRequest{
		Model:    "gpt-5.1",
		Messages: []NormalizedMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	}
	deltaCh, errCh, err := client.UnifiedStreamNormalizedParsed(testCtx(t), req)
	if err != nil {
		t.Fatalf("UnifiedStreamNormalizedParsed: %v", err)
	}

	var deltas []NormalizedDelta
	for d := range deltaCh {
		deltas = append(deltas, d)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("stream error: %v", err)
	}

	assertDeltaSequence(t, deltas, DeltaReasoning, DeltaText, DeltaDone)
}

func TestUnifiedStreamNormalizedParsedGemini(t *testing.T) {
	sse := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"thinking\",\"thought\":true}]}}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"answer\"}]},\"finishReason\":\"STOP\"}]}\n\n"

	server, client := newSSETestServer(t, sse)
	defer server.Close()

	req := NormalizedRequest{
		Model:    "gemini-3-flash",
		Messages: []NormalizedMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	}
	deltaCh, errCh, err := client.UnifiedStreamNormalizedParsed(testCtx(t), req)
	if err != nil {
		t.Fatalf("UnifiedStreamNormalizedParsed: %v", err)
	}

	var deltas []NormalizedDelta
	for d := range deltaCh {
		deltas = append(deltas, d)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("stream error: %v", err)
	}

	assertDeltaSequence(t, deltas, DeltaReasoning, DeltaText, DeltaDone)
}
