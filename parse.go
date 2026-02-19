package zen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NormalizedDeltaType identifies what kind of content a NormalizedDelta carries.
type NormalizedDeltaType string

const (
	// DeltaText is a fragment of the assistant's visible reply.
	DeltaText NormalizedDeltaType = "text"
	// DeltaReasoning is a fragment of the model's reasoning / thinking output.
	DeltaReasoning NormalizedDeltaType = "reasoning"
	// DeltaToolCallBegin signals the start of a tool call (Name is set, ArgumentsDelta is empty).
	DeltaToolCallBegin NormalizedDeltaType = "tool_call_begin"
	// DeltaToolCallArgumentsDelta is an incremental JSON fragment of a tool call's arguments.
	DeltaToolCallArgumentsDelta NormalizedDeltaType = "tool_call_arguments_delta"
	// DeltaToolCallDone signals that a tool call is complete (ID, Name, Arguments all set).
	DeltaToolCallDone NormalizedDeltaType = "tool_call_done"
	// DeltaDone signals that the stream has finished (no content fields are set).
	DeltaDone NormalizedDeltaType = "done"
	// DeltaUnknown is emitted for events that carry no recognized content.
	DeltaUnknown NormalizedDeltaType = "unknown"
)

// NormalizedDelta is a single parsed increment from a streaming response, endpoint-agnostic.
type NormalizedDelta struct {
	Type NormalizedDeltaType

	// Text/Reasoning content (set for DeltaText and DeltaReasoning).
	Content string

	// Tool call fields.
	ToolCallIndex     int    // index within this response (0-based)
	ToolCallID        string // set on DeltaToolCallBegin / DeltaToolCallDone
	ToolCallName      string // set on DeltaToolCallBegin / DeltaToolCallDone
	ToolCallSignature string // set on DeltaToolCallBegin / DeltaToolCallDone when provided by provider
	ArgumentsDelta    string // set on DeltaToolCallArgumentsDelta
	ArgumentsFull     string // set on DeltaToolCallDone (fully accumulated)
}

// ParseNormalizedEvent parses a single UnifiedEvent into zero or more NormalizedDelta values.
// Callers should call this for every event emitted by StreamEvents and accumulate
// the results. Multiple deltas can be returned from a single event (e.g. Anthropic emits a
// content_block_start followed by content_block_delta in separate events, but a Gemini chunk
// may carry both a thought part and a text part).
func ParseNormalizedEvent(ev UnifiedEvent) []NormalizedDelta {
	if len(ev.Data) == 0 {
		return nil
	}

	switch ev.Endpoint {
	case EndpointChatCompletions:
		return parseChatCompletionsDelta(ev)
	case EndpointResponses:
		return parseResponsesDelta(ev)
	case EndpointMessages:
		return parseMessagesDelta(ev)
	case EndpointModels:
		return parseGeminiDelta(ev)
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// chat/completions
// ---------------------------------------------------------------------------

// chatCompletionChunk is the minimal shape of a streaming chat completion chunk.
type chatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
			ReasoningDetails []struct {
				Text string `json:"text"`
			} `json:"reasoning_details"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func parseChatCompletionsDelta(ev UnifiedEvent) []NormalizedDelta {
	var chunk chatCompletionChunk
	if err := json.Unmarshal(ev.Data, &chunk); err != nil {
		return nil
	}
	if len(chunk.Choices) == 0 {
		return nil
	}

	delta := chunk.Choices[0].Delta
	var out []NormalizedDelta

	if delta.ReasoningContent != "" {
		out = append(out, NormalizedDelta{Type: DeltaReasoning, Content: delta.ReasoningContent})
	}
	if delta.Reasoning != "" {
		out = append(out, NormalizedDelta{Type: DeltaReasoning, Content: delta.Reasoning})
	}
	for _, detail := range delta.ReasoningDetails {
		if detail.Text != "" {
			out = append(out, NormalizedDelta{Type: DeltaReasoning, Content: detail.Text})
		}
	}
	if delta.Content != "" {
		out = append(out, NormalizedDelta{Type: DeltaText, Content: delta.Content})
	}
	for _, tc := range delta.ToolCalls {
		if tc.Function.Name != "" || tc.ID != "" {
			out = append(out, NormalizedDelta{
				Type:          DeltaToolCallBegin,
				ToolCallIndex: tc.Index,
				ToolCallID:    tc.ID,
				ToolCallName:  tc.Function.Name,
			})
		}
		if tc.Function.Arguments != "" {
			out = append(out, NormalizedDelta{
				Type:           DeltaToolCallArgumentsDelta,
				ToolCallIndex:  tc.Index,
				ArgumentsDelta: tc.Function.Arguments,
			})
		}
	}

	if chunk.Choices[0].FinishReason != "" {
		out = append(out, NormalizedDelta{Type: DeltaDone})
	}

	return out
}

// ---------------------------------------------------------------------------
// responses (OpenAI Responses API)
// ---------------------------------------------------------------------------

// responsesEvent is the minimal shape of a typed Responses API SSE payload.
type responsesEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
	// For tool call events.
	Item struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		CallID string `json:"call_id"`
		Name   string `json:"name"`
	} `json:"item"`
	OutputIndex int    `json:"output_index"`
	Name        string `json:"name"`
	CallID      string `json:"call_id"`
	ItemID      string `json:"item_id"`
	Arguments   string `json:"arguments"`
}

func parseResponsesDelta(ev UnifiedEvent) []NormalizedDelta {
	var e responsesEvent
	if err := json.Unmarshal(ev.Data, &e); err != nil {
		return nil
	}

	switch e.Type {
	case "response.output_text.delta":
		if e.Delta != "" {
			return []NormalizedDelta{{Type: DeltaText, Content: e.Delta}}
		}
	case "response.reasoning_summary_text.delta", "response.reasoning.delta", "response.reasoning_text.delta":
		if e.Delta != "" {
			return []NormalizedDelta{{Type: DeltaReasoning, Content: e.Delta}}
		}
	case "response.function_call_arguments_delta", "response.function_call_arguments.delta":
		if e.Delta != "" {
			return []NormalizedDelta{{
				Type:           DeltaToolCallArgumentsDelta,
				ToolCallIndex:  e.OutputIndex,
				ArgumentsDelta: e.Delta,
			}}
		}
	case "response.function_call_arguments_done", "response.function_call_arguments.done":
		callID := e.CallID
		if callID == "" {
			callID = e.ItemID
		}
		return []NormalizedDelta{{
			Type:          DeltaToolCallDone,
			ToolCallIndex: e.OutputIndex,
			ToolCallID:    callID,
			ToolCallName:  e.Name,
			ArgumentsFull: e.Arguments,
		}}
	case "response.output_item.added":
		if e.Item.Type == "function_call" {
			callID := e.Item.CallID
			name := e.Item.Name
			if callID == "" || name == "" {
				fallbackCallID, fallbackName := extractResponsesFunctionCallItem(ev.Data)
				if callID == "" {
					callID = fallbackCallID
				}
				if name == "" {
					name = fallbackName
				}
			}
			if callID == "" {
				callID = e.Item.ID
			}
			if name == "" {
				name = e.Name
			}
			return []NormalizedDelta{{
				Type:          DeltaToolCallBegin,
				ToolCallIndex: e.OutputIndex,
				ToolCallID:    callID,
				ToolCallName:  name,
			}}
		}
	case "response.completed", "response.done":
		return []NormalizedDelta{{Type: DeltaDone}}
	}

	return nil
}

func extractResponsesFunctionCallItem(raw json.RawMessage) (string, string) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", ""
	}
	item, ok := root["item"].(map[string]any)
	if !ok {
		return "", ""
	}
	if t, ok := item["type"].(string); ok && t != "function_call" {
		return "", ""
	}

	var callID string
	if v, ok := item["call_id"].(string); ok {
		callID = v
	} else if v, ok := item["id"].(string); ok {
		callID = v
	}

	var name string
	if v, ok := item["name"].(string); ok {
		name = v
	} else if v, ok := item["tool_name"].(string); ok {
		name = v
	}

	return callID, name
}

// ---------------------------------------------------------------------------
// messages (Anthropic)
// ---------------------------------------------------------------------------

// anthropicStreamEvent is the minimal shape of an Anthropic SSE payload.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
		// For tool use input deltas.
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	ContentBlock struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Name  string `json:"name"`
		Input string `json:"input"`
	} `json:"content_block"`
}

func parseMessagesDelta(ev UnifiedEvent) []NormalizedDelta {
	// Anthropic uses the SSE "event:" line for the type, but also includes
	// "type" in the JSON body. Both are supported.
	var e anthropicStreamEvent
	if err := json.Unmarshal(ev.Data, &e); err != nil {
		return nil
	}

	// Prefer the JSON body "type" field; fall back to the SSE event name.
	evType := e.Type
	if evType == "" {
		evType = ev.Event
	}

	switch evType {
	case "content_block_start":
		if e.ContentBlock.Type == "tool_use" {
			return []NormalizedDelta{{
				Type:          DeltaToolCallBegin,
				ToolCallIndex: e.Index,
				ToolCallID:    e.ContentBlock.ID,
				ToolCallName:  e.ContentBlock.Name,
			}}
		}
	case "content_block_delta":
		switch e.Delta.Type {
		case "text_delta":
			if e.Delta.Text != "" {
				return []NormalizedDelta{{Type: DeltaText, Content: e.Delta.Text}}
			}
		case "thinking_delta":
			if e.Delta.Thinking != "" {
				return []NormalizedDelta{{Type: DeltaReasoning, Content: e.Delta.Thinking}}
			}
		case "input_json_delta":
			if e.Delta.PartialJSON != "" {
				return []NormalizedDelta{{
					Type:           DeltaToolCallArgumentsDelta,
					ToolCallIndex:  e.Index,
					ArgumentsDelta: e.Delta.PartialJSON,
				}}
			}
		}
	case "content_block_stop":
		// No content; signal completion only for tool_use blocks would require
		// state from content_block_start. Consumers that need DeltaToolCallDone
		// should accumulate themselves. We emit nothing here.
	case "message_stop":
		return []NormalizedDelta{{Type: DeltaDone}}
	}

	return nil
}

// ---------------------------------------------------------------------------
// models (Gemini)
// ---------------------------------------------------------------------------

// geminiChunk is the minimal shape of a Gemini SSE chunk.
type geminiChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text             string    `json:"text"`
				Thought          bool      `json:"thought"`
				FunctionCall     *geminiFC `json:"functionCall,omitempty"`
				ThoughtSignature string    `json:"thoughtSignature"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

type geminiFC struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

func parseGeminiDelta(ev UnifiedEvent) []NormalizedDelta {
	var chunk geminiChunk
	if err := json.Unmarshal(ev.Data, &chunk); err != nil {
		return nil
	}
	if len(chunk.Candidates) == 0 {
		return nil
	}

	cand := chunk.Candidates[0]
	var out []NormalizedDelta

	for i, part := range cand.Content.Parts {
		if part.FunctionCall != nil {
			callID := fmt.Sprintf("gemini-%d", i)
			callID = encodeGeminiToolCallID(callID, part.ThoughtSignature)
			out = append(out, NormalizedDelta{
				Type:              DeltaToolCallBegin,
				ToolCallIndex:     i,
				ToolCallID:        callID,
				ToolCallName:      part.FunctionCall.Name,
				ToolCallSignature: part.ThoughtSignature,
			})
			if len(part.FunctionCall.Args) > 0 {
				args := string(part.FunctionCall.Args)
				out = append(out, NormalizedDelta{
					Type:           DeltaToolCallArgumentsDelta,
					ToolCallIndex:  i,
					ToolCallID:     callID,
					ArgumentsDelta: args,
				})
				out = append(out, NormalizedDelta{
					Type:              DeltaToolCallDone,
					ToolCallIndex:     i,
					ToolCallID:        callID,
					ToolCallName:      part.FunctionCall.Name,
					ToolCallSignature: part.ThoughtSignature,
					ArgumentsFull:     args,
				})
			}
			continue
		}
		text := strings.TrimRight(part.Text, "")
		if text == "" {
			continue
		}
		if part.Thought {
			out = append(out, NormalizedDelta{Type: DeltaReasoning, Content: text})
		} else {
			out = append(out, NormalizedDelta{Type: DeltaText, Content: text})
		}
	}

	if cand.FinishReason != "" && cand.FinishReason != "FINISH_REASON_UNSPECIFIED" {
		out = append(out, NormalizedDelta{Type: DeltaDone})
	}

	return out
}
