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

type GeminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type GeminiFunctionResponse struct {
	Name     string                     `json:"name"`
	Response GeminiFunctionResponseBody `json:"response"`
}

// GeminiFunctionResponseBody is the value of functionResponse.response sent to
// Gemini. When Content is non-nil it is used verbatim (allowing callers to
// provide structured JSON). Otherwise a plain {"output": <Output>} object is
// marshalled, which is the simplest form accepted by the API.
type GeminiFunctionResponseBody struct {
	Output  string          `json:"output,omitempty"`
	Content json.RawMessage `json:"-"` // if set, marshalled as the entire response object
}

func (b GeminiFunctionResponseBody) MarshalJSON() ([]byte, error) {
	if len(b.Content) > 0 {
		return b.Content, nil
	}
	return json.Marshal(struct {
		Output string `json:"output"`
	}{Output: b.Output})
}

type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
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
	// Note: Gemini streaming is controlled by the URL (:streamGenerateContent?alt=sse),
	// not a body field. The Stream field is intentionally omitted here.

	return marshalWithExtra(base, r.Extra)
}
