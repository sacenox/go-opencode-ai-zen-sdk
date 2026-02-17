package zen

import "encoding/json"

type GeminiRequest struct {
	Contents          []GeminiContent
	SystemInstruction *GeminiContent
	GenerationConfig  *GeminiGenerationConfig
	Tools             []GeminiTool
	ToolConfig        *GeminiToolConfig
	Stream            bool
	Extra             map[string]any
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type GeminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type GeminiToolConfig struct {
	FunctionCallingConfig *GeminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type GeminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type GeminiGenerationConfig struct {
	Temperature     *float64              `json:"temperature,omitempty"`
	MaxOutputTokens *int                  `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GeminiThinkingConfig struct {
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

func (r GeminiRequest) MarshalJSON() ([]byte, error) {
	base := map[string]any{
		"contents": r.Contents,
	}
	if r.SystemInstruction != nil {
		base["systemInstruction"] = r.SystemInstruction
	}
	if r.GenerationConfig != nil {
		base["generationConfig"] = r.GenerationConfig
	}
	if len(r.Tools) > 0 {
		base["tools"] = r.Tools
	}
	if r.ToolConfig != nil {
		base["toolConfig"] = r.ToolConfig
	}
	if r.Stream {
		base["stream"] = r.Stream
	}

	return marshalWithExtra(base, r.Extra)
}
