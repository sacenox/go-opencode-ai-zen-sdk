package zen

import "testing"

func TestMapOpenAIToolChoice(t *testing.T) {
	_, err := mapOpenAIToolChoice(NormalizedToolChoice{Type: ToolChoiceTool})
	if err == nil {
		t.Fatalf("expected error for missing tool name")
	}

	choice, err := mapOpenAIToolChoice(NormalizedToolChoice{Type: ToolChoiceAuto})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != "auto" {
		t.Fatalf("expected auto, got %v", choice)
	}
}

func TestMapAnthropicToolChoice(t *testing.T) {
	choice, err := mapAnthropicToolChoice(NormalizedToolChoice{Type: ToolChoiceNone})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != nil {
		t.Fatalf("expected nil choice for none")
	}
}

func TestMapGeminiToolChoice(t *testing.T) {
	_, err := mapGeminiToolChoice(NormalizedToolChoice{Type: ToolChoiceTool})
	if err == nil {
		t.Fatalf("expected error for missing tool name")
	}

	choice, err := mapGeminiToolChoice(NormalizedToolChoice{Type: ToolChoiceRequired})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice == nil || choice.FunctionCallingConfig == nil || choice.FunctionCallingConfig.Mode != "ANY" {
		t.Fatalf("expected ANY mode")
	}
}

func TestNormalizeAnthropicMessages(t *testing.T) {
	system, msgs := normalizeAnthropicMessages("base", []NormalizedMessage{
		{Role: "system", Content: "sys"},
		{Role: "developer", Content: "dev"},
		{Role: "user", Content: "hi"},
	})
	if system == "" {
		t.Fatalf("expected system to be combined")
	}
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatalf("expected only user message")
	}
}

func TestMapEffortToBudget(t *testing.T) {
	if mapEffortToBudget("low") == 0 {
		t.Fatalf("expected non-zero budget")
	}
	if mapEffortToBudget("unknown") != 0 {
		t.Fatalf("expected zero budget")
	}
}

func TestMapEffortToThinkingLevel(t *testing.T) {
	if mapEffortToThinkingLevel("medium") != "medium" {
		t.Fatalf("expected medium level")
	}
	if mapEffortToThinkingLevel("unknown") != "" {
		t.Fatalf("expected empty level")
	}
}
