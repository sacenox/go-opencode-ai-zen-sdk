package zen

import "encoding/json"

type MessagesRequest struct {
	Model       string
	System      string
	Messages    []AnthropicMessage
	Tools       []AnthropicTool
	ToolChoice  *AnthropicToolChoice
	Thinking    *AnthropicThinking
	Temperature *float64
	MaxTokens   *int
	Stream      bool
	Extra       map[string]any
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type AnthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

func (r MessagesRequest) MarshalJSON() ([]byte, error) {
	base := map[string]any{
		"model":    r.Model,
		"messages": r.Messages,
	}
	if r.System != "" {
		base["system"] = r.System
	}
	if len(r.Tools) > 0 {
		base["tools"] = r.Tools
	}
	if r.ToolChoice != nil {
		base["tool_choice"] = r.ToolChoice
	}
	if r.Thinking != nil {
		base["thinking"] = r.Thinking
	}
	if r.Temperature != nil {
		base["temperature"] = r.Temperature
	}
	if r.MaxTokens != nil {
		base["max_tokens"] = r.MaxTokens
	}
	if r.Stream {
		base["stream"] = r.Stream
	}

	return marshalWithExtra(base, r.Extra)
}
