package zen

import "encoding/json"

type ChatCompletionsRequest struct {
	Model       string
	Messages    []ChatMessage
	Reasoning   *ChatReasoning
	Tools       []ChatTool
	ToolChoice  any
	Temperature *float64
	MaxTokens   *int
	Stream      bool
	Extra       map[string]any
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type ChatTool struct {
	Type     string           `json:"type"`
	Function ChatToolFunction `json:"function"`
}

type ChatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func (r ChatCompletionsRequest) MarshalJSON() ([]byte, error) {
	base := map[string]any{
		"model":    r.Model,
		"messages": r.Messages,
	}
	if r.Reasoning != nil {
		base["reasoning"] = r.Reasoning
	}
	if len(r.Tools) > 0 {
		base["tools"] = r.Tools
	}
	if r.ToolChoice != nil {
		base["tool_choice"] = r.ToolChoice
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
