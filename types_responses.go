package zen

import "encoding/json"

type ResponsesRequest struct {
	Model           string
	Input           any
	Instructions    string
	Reasoning       *ResponsesReasoning
	Tools           []ResponsesTool
	ToolChoice      any
	Temperature     *float64
	MaxOutputTokens *int
	Stream          bool
	Extra           map[string]any
}

type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// ResponsesTool defines a function tool in the Responses API flat format:
// {"type":"function","name":...,"description":...,"parameters":...}
// Note: this differs from Chat Completions which nests these under a "function" key.
type ResponsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type ResponsesInputMessage struct {
	Role    string                  `json:"role"`
	Content []ResponsesInputContent `json:"content"`
}

type ResponsesInputContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ResponsesFunctionCall represents an assistant function-call output item in
// the Responses API input array.
type ResponsesFunctionCall struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponsesFunctionCallOutput represents a tool result item in the Responses
// API input array.
type ResponsesFunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

func (r ResponsesRequest) MarshalJSON() ([]byte, error) {
	base := map[string]any{
		"model": r.Model,
	}
	if r.Input != nil {
		base["input"] = r.Input
	}
	if r.Instructions != "" {
		base["instructions"] = r.Instructions
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
	if r.MaxOutputTokens != nil {
		base["max_output_tokens"] = r.MaxOutputTokens
	}
	if r.Stream {
		base["stream"] = r.Stream
	}

	return marshalWithExtra(base, r.Extra)
}
