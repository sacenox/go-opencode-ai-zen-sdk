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

type NormalizedToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type NormalizedMessage struct {
	Role         string
	Content      string
	ToolCalls    []NormalizedToolCall // set on assistant messages that invoked tools
	ToolCallID   string               // set on tool-result messages (role "tool")
	FunctionName string               // set on tool-result messages (role "tool"): name of the called function; required by Gemini
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
		items := make([]any, 0, len(r.Messages))
		for _, m := range r.Messages {
			// Tool result message → function_call_output item.
			if strings.ToLower(strings.TrimSpace(m.Role)) == "tool" {
				items = append(items, ResponsesFunctionCallOutput{
					Type:   "function_call_output",
					CallID: m.ToolCallID,
					Output: m.Content,
				})
				continue
			}
			// Assistant message with tool calls → optional text item + function_call items.
			if strings.ToLower(strings.TrimSpace(m.Role)) == "assistant" && len(m.ToolCalls) > 0 {
				if strings.TrimSpace(m.Content) != "" {
					items = append(items, ResponsesInputMessage{Role: m.Role, Content: m.Content})
				}
				for _, tc := range m.ToolCalls {
					items = append(items, ResponsesFunctionCall{
						Type:      "function_call",
						CallID:    tc.ID,
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					})
				}
				continue
			}
			items = append(items, ResponsesInputMessage{Role: m.Role, Content: m.Content})
		}
		req.Input = items
	}

	if r.Reasoning != nil && r.Reasoning.Effort != "" {
		req.Reasoning = &ResponsesReasoning{Effort: r.Reasoning.Effort}
	}

	if len(r.Tools) > 0 {
		req.Tools = make([]ResponsesTool, 0, len(r.Tools))
		for _, t := range r.Tools {
			req.Tools = append(req.Tools, ResponsesTool{
				Type:        "function",
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
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
		cm := ChatMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		if len(m.ToolCalls) > 0 {
			cm.ToolCalls = make([]ChatMessageToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				cm.ToolCalls = append(cm.ToolCalls, ChatMessageToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: ChatMessageToolCallFunc{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				})
			}
		}
		messages = append(messages, cm)
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

	// Anthropic's messages API requires max_tokens; apply a default when the
	// caller did not specify one so the normalized path works out of the box.
	maxTokens := r.MaxTokens

	// Resolve the thinking budget early so we can ensure max_tokens > budget_tokens,
	// which is required by Anthropic's API.
	var thinkingBudget int
	if r.Reasoning != nil {
		thinkingBudget = r.Reasoning.BudgetTokens
		if thinkingBudget == 0 && r.Reasoning.Effort != "" {
			thinkingBudget = mapEffortToBudget(r.Reasoning.Effort)
		}
	}

	if maxTokens == nil {
		defaultMax := 1024
		// When reasoning is active, max_tokens must be strictly greater than
		// budget_tokens. Default to twice the budget so there is room for the
		// actual response text.
		if thinkingBudget > 0 && thinkingBudget >= defaultMax {
			defaultMax = thinkingBudget * 2
		}
		maxTokens = &defaultMax
	}

	req := &MessagesRequest{
		Model:       r.Model,
		System:      system,
		Messages:    messages,
		Temperature: r.Temperature,
		MaxTokens:   maxTokens,
		Stream:      r.Stream,
		Extra:       r.Extra,
	}

	if thinkingBudget > 0 {
		req.Thinking = &AnthropicThinking{Type: "enabled", BudgetTokens: thinkingBudget}
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
	// Build a call-id → function-name index from all assistant tool calls so
	// that tool-result messages can have their FunctionName derived
	// automatically when the caller did not set it explicitly.
	callIDToName := make(map[string]string)
	for _, m := range r.Messages {
		if strings.ToLower(strings.TrimSpace(m.Role)) == "assistant" {
			for _, tc := range m.ToolCalls {
				if tc.ID != "" && tc.Name != "" {
					callIDToName[tc.ID] = tc.Name
				}
			}
		}
	}

	contents := make([]GeminiContent, 0, len(r.Messages))
	for _, m := range r.Messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))

		// Tool result: role "tool" → user turn with functionResponse parts.
		if role == "tool" {
			name := m.FunctionName
			if name == "" {
				name = callIDToName[m.ToolCallID]
			}
			if name == "" {
				return nil, errors.New("zen: tool result message is missing FunctionName (required by Gemini)")
			}
			contents = append(contents, GeminiContent{
				Role: "user",
				Parts: []GeminiPart{{
					FunctionResponse: &GeminiFunctionResponse{
						Name:     name,
						Response: GeminiFunctionResponseBody{Output: m.Content},
					},
				}},
			})
			continue
		}

		if role == "assistant" {
			role = "model"
		}

		// Assistant message with tool calls → model turn with functionCall parts.
		if role == "model" && len(m.ToolCalls) > 0 {
			parts := make([]GeminiPart, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				parts = append(parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{
						Name: tc.Name,
						Args: tc.Arguments,
					},
				})
			}
			contents = append(contents, GeminiContent{Role: role, Parts: parts})
			continue
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

		// Tool result: role "tool" maps to a "user" message with a tool_result block.
		if role == "tool" {
			out = append(out, AnthropicMessage{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content},
				},
			})
			continue
		}

		// Assistant message with tool calls: emit content blocks of type "tool_use".
		if role == "assistant" && len(m.ToolCalls) > 0 {
			blocks := make([]AnthropicContentBlock, 0, len(m.ToolCalls)+1)
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Arguments,
				})
			}
			out = append(out, AnthropicMessage{Role: "assistant", Content: blocks})
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
