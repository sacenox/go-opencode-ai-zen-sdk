package zen

import (
	"encoding/json"
	"errors"
	"strings"
)

type ToolChoiceType string

const (
	ToolChoiceAuto     ToolChoiceType = "auto"
	ToolChoiceNone     ToolChoiceType = "none"
	ToolChoiceRequired ToolChoiceType = "required"
	ToolChoiceTool     ToolChoiceType = "tool"
)

type NormalizedToolChoice struct {
	Type ToolChoiceType
	Name string
}

type NormalizedTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type NormalizedReasoning struct {
	Effort       string
	BudgetTokens int
}

type NormalizedMessage struct {
	Role    string
	Content string
}

type NormalizedRequest struct {
	Model       string
	System      string
	Messages    []NormalizedMessage
	Tools       []NormalizedTool
	ToolChoice  *NormalizedToolChoice
	Reasoning   *NormalizedReasoning
	Temperature *float64
	MaxTokens   *int
	Stream      bool
	Endpoint    EndpointType
	Extra       map[string]any
}

func (r NormalizedRequest) ToResponsesRequest() (*ResponsesRequest, error) {
	req := &ResponsesRequest{
		Model:           r.Model,
		Instructions:    r.System,
		Temperature:     r.Temperature,
		MaxOutputTokens: r.MaxTokens,
		Stream:          r.Stream,
		Extra:           r.Extra,
	}

	if len(r.Messages) == 0 {
		req.Input = ""
	} else {
		messages := make([]ResponsesInputMessage, 0, len(r.Messages))
		for _, m := range r.Messages {
			messages = append(messages, ResponsesInputMessage(m))
		}
		req.Input = messages
	}

	if r.Reasoning != nil && r.Reasoning.Effort != "" {
		req.Reasoning = &ResponsesReasoning{Effort: r.Reasoning.Effort}
	}

	if len(r.Tools) > 0 {
		req.Tools = make([]ResponsesTool, 0, len(r.Tools))
		for _, t := range r.Tools {
			req.Tools = append(req.Tools, ResponsesTool{
				Type:     "function",
				Function: ResponsesToolFunction(t),
			})
		}
	}

	if r.ToolChoice != nil {
		choice, err := mapOpenAIToolChoice(*r.ToolChoice)
		if err != nil {
			return nil, err
		}
		req.ToolChoice = choice
	}

	return req, nil
}

func (r NormalizedRequest) ToChatCompletionsRequest() (*ChatCompletionsRequest, error) {
	messages := make([]ChatMessage, 0, len(r.Messages)+1)
	if strings.TrimSpace(r.System) != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: r.System})
	}
	for _, m := range r.Messages {
		messages = append(messages, ChatMessage(m))
	}

	req := &ChatCompletionsRequest{
		Model:       r.Model,
		Messages:    messages,
		Temperature: r.Temperature,
		MaxTokens:   r.MaxTokens,
		Stream:      r.Stream,
		Extra:       r.Extra,
	}

	if r.Reasoning != nil && r.Reasoning.Effort != "" {
		req.Reasoning = &ChatReasoning{Effort: r.Reasoning.Effort}
	}

	if len(r.Tools) > 0 {
		req.Tools = make([]ChatTool, 0, len(r.Tools))
		for _, t := range r.Tools {
			req.Tools = append(req.Tools, ChatTool{
				Type:     "function",
				Function: ChatToolFunction(t),
			})
		}
	}

	if r.ToolChoice != nil {
		choice, err := mapOpenAIToolChoice(*r.ToolChoice)
		if err != nil {
			return nil, err
		}
		req.ToolChoice = choice
	}

	return req, nil
}

func (r NormalizedRequest) ToMessagesRequest() (*MessagesRequest, error) {
	system, messages := normalizeAnthropicMessages(r.System, r.Messages)
	req := &MessagesRequest{
		Model:       r.Model,
		System:      system,
		Messages:    messages,
		Temperature: r.Temperature,
		MaxTokens:   r.MaxTokens,
		Stream:      r.Stream,
		Extra:       r.Extra,
	}

	if r.Reasoning != nil {
		budget := r.Reasoning.BudgetTokens
		if budget == 0 && r.Reasoning.Effort != "" {
			budget = mapEffortToBudget(r.Reasoning.Effort)
		}
		if budget > 0 {
			req.Thinking = &AnthropicThinking{Type: "enabled", BudgetTokens: budget}
		}
	}

	if r.ToolChoice != nil && r.ToolChoice.Type == ToolChoiceNone {
		return req, nil
	}

	if len(r.Tools) > 0 {
		req.Tools = make([]AnthropicTool, 0, len(r.Tools))
		for _, t := range r.Tools {
			req.Tools = append(req.Tools, AnthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			})
		}
	}

	if r.ToolChoice != nil {
		choice, err := mapAnthropicToolChoice(*r.ToolChoice)
		if err != nil {
			return nil, err
		}
		req.ToolChoice = choice
	}

	return req, nil
}

func (r NormalizedRequest) ToGeminiRequest() (*GeminiRequest, error) {
	contents := make([]GeminiContent, 0, len(r.Messages))
	for _, m := range r.Messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: m.Content}},
		})
	}

	var systemInstruction *GeminiContent
	if strings.TrimSpace(r.System) != "" {
		systemInstruction = &GeminiContent{
			Role:  "system",
			Parts: []GeminiPart{{Text: r.System}},
		}
	}

	req := &GeminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		Stream:            r.Stream,
		Extra:             r.Extra,
	}

	config := &GeminiGenerationConfig{
		Temperature:     r.Temperature,
		MaxOutputTokens: r.MaxTokens,
	}
	if r.Reasoning != nil && r.Reasoning.Effort != "" {
		level := mapEffortToThinkingLevel(r.Reasoning.Effort)
		if level != "" {
			config.ThinkingConfig = &GeminiThinkingConfig{ThinkingLevel: level}
		}
	}
	if config.Temperature != nil || config.MaxOutputTokens != nil || config.ThinkingConfig != nil {
		req.GenerationConfig = config
	}

	if len(r.Tools) > 0 {
		tool := GeminiTool{FunctionDeclarations: make([]GeminiFunctionDeclaration, 0, len(r.Tools))}
		for _, t := range r.Tools {
			tool.FunctionDeclarations = append(tool.FunctionDeclarations, GeminiFunctionDeclaration(t))
		}
		req.Tools = []GeminiTool{tool}
	}

	if r.ToolChoice != nil {
		config, err := mapGeminiToolChoice(*r.ToolChoice)
		if err != nil {
			return nil, err
		}
		if config != nil {
			req.ToolConfig = config
		}
	}

	return req, nil
}

func mapOpenAIToolChoice(choice NormalizedToolChoice) (any, error) {
	switch choice.Type {
	case ToolChoiceAuto:
		return "auto", nil
	case ToolChoiceNone:
		return "none", nil
	case ToolChoiceRequired:
		return "required", nil
	case ToolChoiceTool:
		if strings.TrimSpace(choice.Name) == "" {
			return nil, errors.New("zen: tool choice name is required")
		}
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": choice.Name,
			},
		}, nil
	default:
		return nil, errors.New("zen: unsupported tool choice")
	}
}

func mapAnthropicToolChoice(choice NormalizedToolChoice) (*AnthropicToolChoice, error) {
	switch choice.Type {
	case ToolChoiceAuto:
		return &AnthropicToolChoice{Type: "auto"}, nil
	case ToolChoiceRequired:
		return &AnthropicToolChoice{Type: "any"}, nil
	case ToolChoiceTool:
		if strings.TrimSpace(choice.Name) == "" {
			return nil, errors.New("zen: tool choice name is required")
		}
		return &AnthropicToolChoice{Type: "tool", Name: choice.Name}, nil
	case ToolChoiceNone:
		return nil, nil
	default:
		return nil, errors.New("zen: unsupported tool choice")
	}
}

func normalizeAnthropicMessages(system string, msgs []NormalizedMessage) (string, []AnthropicMessage) {
	combinedSystem := strings.TrimSpace(system)
	out := make([]AnthropicMessage, 0, len(msgs))

	for _, m := range msgs {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "system" || role == "developer" {
			if strings.TrimSpace(m.Content) != "" {
				if combinedSystem != "" {
					combinedSystem += "\n\n"
				}
				combinedSystem += m.Content
			}
			continue
		}

		out = append(out, AnthropicMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	return combinedSystem, out
}

func mapEffortToBudget(effort string) int {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low":
		return 1024
	case "medium":
		return 2048
	case "high":
		return 4096
	default:
		return 0
	}
}

func mapEffortToThinkingLevel(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low":
		return "LOW"
	case "medium":
		return "MEDIUM"
	case "high":
		return "HIGH"
	default:
		return ""
	}
}

func mapGeminiToolChoice(choice NormalizedToolChoice) (*GeminiToolConfig, error) {
	switch choice.Type {
	case ToolChoiceAuto:
		return &GeminiToolConfig{FunctionCallingConfig: &GeminiFunctionCallingConfig{Mode: "AUTO"}}, nil
	case ToolChoiceNone:
		return &GeminiToolConfig{FunctionCallingConfig: &GeminiFunctionCallingConfig{Mode: "NONE"}}, nil
	case ToolChoiceRequired:
		return &GeminiToolConfig{FunctionCallingConfig: &GeminiFunctionCallingConfig{Mode: "ANY"}}, nil
	case ToolChoiceTool:
		if strings.TrimSpace(choice.Name) == "" {
			return nil, errors.New("zen: tool choice name is required")
		}
		return &GeminiToolConfig{FunctionCallingConfig: &GeminiFunctionCallingConfig{Mode: "ANY", AllowedFunctionNames: []string{choice.Name}}}, nil
	default:
		return nil, errors.New("zen: unsupported tool choice")
	}
}
