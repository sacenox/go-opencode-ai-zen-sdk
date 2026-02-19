package zen

import (
	"encoding/json"
	"testing"
)

// toolHistory is a two-turn tool-use history shared across all endpoint tests:
//
//	user → assistant+tool_calls → tool result → assistant
var toolHistory = []NormalizedMessage{
	{Role: "user", Content: "What's the weather in Paris?"},
	{
		Role:    "assistant",
		Content: "",
		ToolCalls: []NormalizedToolCall{
			{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"Paris"}`)},
		},
	},
	// FunctionName is intentionally omitted: ToGeminiRequest derives it from
	// the preceding assistant message's ToolCalls via ToolCallID.
	{Role: "tool", Content: "Sunny, 22°C", ToolCallID: "call_1"},
	{Role: "assistant", Content: "The weather in Paris is sunny and 22°C."},
}

func TestNormalizedToResponsesToolHistory(t *testing.T) {
	req := NormalizedRequest{
		Model:    "gpt-5.2-codex",
		Messages: toolHistory,
	}

	resp, err := req.ToResponsesRequest()
	if err != nil {
		t.Fatalf("ToResponsesRequest error: %v", err)
	}

	items, ok := resp.Input.([]any)
	if !ok {
		t.Fatalf("Input is not []any, got %T", resp.Input)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 input items, got %d", len(items))
	}

	// [0] user message
	userMsg, ok := items[0].(ResponsesInputMessage)
	if !ok || userMsg.Role != "user" {
		t.Fatalf("item[0] should be user ResponsesInputMessage, got %T %+v", items[0], items[0])
	}

	// [1] function_call item for assistant tool call
	fc, ok := items[1].(ResponsesFunctionCall)
	if !ok {
		t.Fatalf("item[1] should be ResponsesFunctionCall, got %T %+v", items[1], items[1])
	}
	if fc.Type != "function_call" {
		t.Fatalf("item[1] type: want function_call, got %q", fc.Type)
	}
	if fc.CallID != "call_1" {
		t.Fatalf("item[1] call_id: want call_1, got %q", fc.CallID)
	}
	if fc.Name != "get_weather" {
		t.Fatalf("item[1] name: want get_weather, got %q", fc.Name)
	}

	// [2] function_call_output item for tool result
	fco, ok := items[2].(ResponsesFunctionCallOutput)
	if !ok {
		t.Fatalf("item[2] should be ResponsesFunctionCallOutput, got %T %+v", items[2], items[2])
	}
	if fco.Type != "function_call_output" {
		t.Fatalf("item[2] type: want function_call_output, got %q", fco.Type)
	}
	if fco.CallID != "call_1" {
		t.Fatalf("item[2] call_id: want call_1, got %q", fco.CallID)
	}
	if fco.Output != "Sunny, 22°C" {
		t.Fatalf("item[2] output: want %q, got %q", "Sunny, 22°C", fco.Output)
	}

	// [3] final assistant message
	assistantMsg, ok := items[3].(ResponsesInputMessage)
	if !ok || assistantMsg.Role != "assistant" {
		t.Fatalf("item[3] should be assistant ResponsesInputMessage, got %T %+v", items[3], items[3])
	}
}

func TestNormalizedToChatCompletionsToolHistory(t *testing.T) {
	req := NormalizedRequest{
		Model:    "gpt-5.2",
		Messages: toolHistory,
	}

	chat, err := req.ToChatCompletionsRequest()
	if err != nil {
		t.Fatalf("ToChatCompletionsRequest error: %v", err)
	}

	// No system message injected (System is empty), so 4 messages.
	if len(chat.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(chat.Messages))
	}

	// [0] user
	if chat.Messages[0].Role != "user" {
		t.Fatalf("msg[0] role: want user, got %q", chat.Messages[0].Role)
	}

	// [1] assistant with tool_calls
	asst := chat.Messages[1]
	if asst.Role != "assistant" {
		t.Fatalf("msg[1] role: want assistant, got %q", asst.Role)
	}
	if len(asst.ToolCalls) != 1 {
		t.Fatalf("msg[1] tool_calls length: want 1, got %d", len(asst.ToolCalls))
	}
	if asst.ToolCalls[0].ID != "call_1" {
		t.Fatalf("msg[1] tool_calls[0].ID: want call_1, got %q", asst.ToolCalls[0].ID)
	}
	if asst.ToolCalls[0].Type != "function" {
		t.Fatalf("msg[1] tool_calls[0].Type: want function, got %q", asst.ToolCalls[0].Type)
	}
	if asst.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("msg[1] tool_calls[0].Function.Name: want get_weather, got %q", asst.ToolCalls[0].Function.Name)
	}
	if asst.ToolCalls[0].Function.Arguments != `{"city":"Paris"}` {
		t.Fatalf("msg[1] tool_calls[0].Function.Arguments: want %q, got %q", `{"city":"Paris"}`, asst.ToolCalls[0].Function.Arguments)
	}

	// [2] tool result
	toolMsg := chat.Messages[2]
	if toolMsg.Role != "tool" {
		t.Fatalf("msg[2] role: want tool, got %q", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "call_1" {
		t.Fatalf("msg[2] tool_call_id: want call_1, got %q", toolMsg.ToolCallID)
	}
	if toolMsg.Content != "Sunny, 22°C" {
		t.Fatalf("msg[2] content: want %q, got %q", "Sunny, 22°C", toolMsg.Content)
	}

	// [3] final assistant
	if chat.Messages[3].Role != "assistant" {
		t.Fatalf("msg[3] role: want assistant, got %q", chat.Messages[3].Role)
	}
}

func TestNormalizedToMessagesToolHistory(t *testing.T) {
	req := NormalizedRequest{
		Model:    "claude-sonnet-4-6",
		Messages: toolHistory,
	}

	msg, err := req.ToMessagesRequest()
	if err != nil {
		t.Fatalf("ToMessagesRequest error: %v", err)
	}

	if len(msg.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msg.Messages))
	}

	// [0] user plain text
	m0 := msg.Messages[0]
	if m0.Role != "user" {
		t.Fatalf("msg[0] role: want user, got %q", m0.Role)
	}
	if m0.Content.(string) != "What's the weather in Paris?" {
		t.Fatalf("msg[0] content mismatch")
	}

	// [1] assistant with tool_use block
	m1 := msg.Messages[1]
	if m1.Role != "assistant" {
		t.Fatalf("msg[1] role: want assistant, got %q", m1.Role)
	}
	blocks1, ok := m1.Content.([]AnthropicContentBlock)
	if !ok {
		t.Fatalf("msg[1] content should be []AnthropicContentBlock, got %T", m1.Content)
	}
	if len(blocks1) != 1 {
		t.Fatalf("msg[1] content blocks: want 1, got %d", len(blocks1))
	}
	if blocks1[0].Type != "tool_use" {
		t.Fatalf("msg[1] block[0].Type: want tool_use, got %q", blocks1[0].Type)
	}
	if blocks1[0].ID != "call_1" {
		t.Fatalf("msg[1] block[0].ID: want call_1, got %q", blocks1[0].ID)
	}
	if blocks1[0].Name != "get_weather" {
		t.Fatalf("msg[1] block[0].Name: want get_weather, got %q", blocks1[0].Name)
	}

	// [2] user with tool_result block (tool role → user message)
	m2 := msg.Messages[2]
	if m2.Role != "user" {
		t.Fatalf("msg[2] role: want user (tool_result), got %q", m2.Role)
	}
	blocks2, ok := m2.Content.([]AnthropicContentBlock)
	if !ok {
		t.Fatalf("msg[2] content should be []AnthropicContentBlock, got %T", m2.Content)
	}
	if len(blocks2) != 1 || blocks2[0].Type != "tool_result" {
		t.Fatalf("msg[2] should have 1 tool_result block, got %+v", blocks2)
	}
	if blocks2[0].ToolUseID != "call_1" {
		t.Fatalf("msg[2] block[0].ToolUseID: want call_1, got %q", blocks2[0].ToolUseID)
	}
	if blocks2[0].Content != "Sunny, 22°C" {
		t.Fatalf("msg[2] block[0].Content: want %q, got %q", "Sunny, 22°C", blocks2[0].Content)
	}

	// [3] final assistant plain text
	m3 := msg.Messages[3]
	if m3.Role != "assistant" {
		t.Fatalf("msg[3] role: want assistant, got %q", m3.Role)
	}
	if m3.Content.(string) != "The weather in Paris is sunny and 22°C." {
		t.Fatalf("msg[3] content mismatch")
	}
}

func TestNormalizedToGeminiToolHistory(t *testing.T) {
	req := NormalizedRequest{
		Model:    "gemini-3-pro",
		Messages: toolHistory,
	}

	gem, err := req.ToGeminiRequest()
	if err != nil {
		t.Fatalf("ToGeminiRequest error: %v", err)
	}

	if len(gem.Contents) != 4 {
		t.Fatalf("expected 4 contents, got %d", len(gem.Contents))
	}

	// [0] user plain text
	c0 := gem.Contents[0]
	if c0.Role != "user" {
		t.Fatalf("content[0] role: want user, got %q", c0.Role)
	}
	if len(c0.Parts) != 1 || c0.Parts[0].Text != "What's the weather in Paris?" {
		t.Fatalf("content[0] parts mismatch: %+v", c0.Parts)
	}

	// [1] model with functionCall part
	c1 := gem.Contents[1]
	if c1.Role != "model" {
		t.Fatalf("content[1] role: want model, got %q", c1.Role)
	}
	if len(c1.Parts) != 1 || c1.Parts[0].FunctionCall == nil {
		t.Fatalf("content[1] should have 1 functionCall part, got %+v", c1.Parts)
	}
	if c1.Parts[0].FunctionCall.Name != "get_weather" {
		t.Fatalf("content[1] functionCall.Name: want get_weather, got %q", c1.Parts[0].FunctionCall.Name)
	}

	// [2] user with functionResponse part
	c2 := gem.Contents[2]
	if c2.Role != "user" {
		t.Fatalf("content[2] role: want user, got %q", c2.Role)
	}
	if len(c2.Parts) != 1 || c2.Parts[0].FunctionResponse == nil {
		t.Fatalf("content[2] should have 1 functionResponse part, got %+v", c2.Parts)
	}
	if c2.Parts[0].FunctionResponse.Name != "get_weather" {
		t.Fatalf("content[2] functionResponse.Name: want get_weather, got %q", c2.Parts[0].FunctionResponse.Name)
	}
	if c2.Parts[0].FunctionResponse.Response.Output != "Sunny, 22°C" {
		t.Fatalf("content[2] functionResponse.Response.Output: want %q, got %q", "Sunny, 22°C", c2.Parts[0].FunctionResponse.Response.Output)
	}

	// [3] final model plain text
	c3 := gem.Contents[3]
	if c3.Role != "model" {
		t.Fatalf("content[3] role: want model, got %q", c3.Role)
	}
	if len(c3.Parts) != 1 || c3.Parts[0].Text != "The weather in Paris is sunny and 22°C." {
		t.Fatalf("content[3] parts mismatch: %+v", c3.Parts)
	}
}

func TestNormalizedToResponses(t *testing.T) {
	req := NormalizedRequest{
		Model:  "gpt-5.2-codex",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
		Tools: []NormalizedTool{
			{Name: "tool", Description: "desc", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolChoice: &NormalizedToolChoice{Type: ToolChoiceTool, Name: "tool"},
		Reasoning:  &NormalizedReasoning{Effort: "low"},
	}

	resp, err := req.ToResponsesRequest()
	if err != nil {
		t.Fatalf("ToResponsesRequest error: %v", err)
	}
	if resp.Model != req.Model {
		t.Fatalf("model mismatch: %s", resp.Model)
	}
	if resp.Instructions != "system" {
		t.Fatalf("instructions mismatch: %s", resp.Instructions)
	}
	if resp.Reasoning == nil || resp.Reasoning.Effort != "low" {
		t.Fatalf("reasoning not mapped")
	}
	if len(resp.Tools) != 1 || resp.Tools[0].Name != "tool" {
		t.Fatalf("tools not mapped")
	}
	choice, ok := resp.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("tool choice type mismatch")
	}
	function, ok := choice["function"].(map[string]any)
	if !ok || function["name"] != "tool" {
		t.Fatalf("tool choice name mismatch")
	}
}

func TestNormalizedToChatCompletions(t *testing.T) {
	req := NormalizedRequest{
		Model:  "gpt-5.2",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
		ToolChoice: &NormalizedToolChoice{Type: ToolChoiceAuto},
	}

	chat, err := req.ToChatCompletionsRequest()
	if err != nil {
		t.Fatalf("ToChatCompletionsRequest error: %v", err)
	}
	if len(chat.Messages) != 2 || chat.Messages[0].Role != "system" {
		t.Fatalf("system message not injected")
	}
	if chat.ToolChoice != "auto" {
		t.Fatalf("tool choice not mapped")
	}
}

func TestNormalizedToMessages(t *testing.T) {
	req := NormalizedRequest{
		Model:  "claude-sonnet-4-6",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
		Reasoning: &NormalizedReasoning{Effort: "medium"},
	}

	msg, err := req.ToMessagesRequest()
	if err != nil {
		t.Fatalf("ToMessagesRequest error: %v", err)
	}
	if msg.System == "" {
		t.Fatalf("system should be set")
	}
	if len(msg.Messages) != 2 {
		t.Fatalf("messages not mapped")
	}
	if msg.Thinking == nil || msg.Thinking.BudgetTokens == 0 {
		t.Fatalf("thinking not mapped")
	}
}

func TestNormalizedToGemini(t *testing.T) {
	req := NormalizedRequest{
		Model:  "gemini-3-pro",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
		ToolChoice: &NormalizedToolChoice{Type: ToolChoiceRequired},
		Reasoning:  &NormalizedReasoning{Effort: "high"},
	}

	gem, err := req.ToGeminiRequest()
	if err != nil {
		t.Fatalf("ToGeminiRequest error: %v", err)
	}
	if gem.SystemInstruction == nil {
		t.Fatalf("system instruction not set")
	}
	if gem.ToolConfig == nil || gem.ToolConfig.FunctionCallingConfig == nil {
		t.Fatalf("tool config missing")
	}
	if gem.ToolConfig.FunctionCallingConfig.Mode != "ANY" {
		t.Fatalf("tool choice not mapped to ANY")
	}
	if gem.GenerationConfig == nil || gem.GenerationConfig.ThinkingConfig == nil {
		t.Fatalf("thinking config missing")
	}
}
