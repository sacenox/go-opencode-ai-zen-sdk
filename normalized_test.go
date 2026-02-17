package zen

import (
	"encoding/json"
	"testing"
)

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
	if len(resp.Tools) != 1 || resp.Tools[0].Function.Name != "tool" {
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
