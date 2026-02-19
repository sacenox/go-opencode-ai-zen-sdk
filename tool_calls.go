package zen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// StreamToolCall is a fully assembled tool call extracted from a stream.
type StreamToolCall struct {
	ID               string
	Name             string
	Arguments        json.RawMessage
	ThoughtSignature string
}

// ToolCallAccumulator stitches streaming tool call deltas into complete calls.
// Call Apply for every NormalizedDelta, then call CompleteCalls at the end.
type ToolCallAccumulator struct {
	calls map[int]*toolCallState
	order []int
}

type toolCallState struct {
	index int
	id    string
	name  string
	sig   string
	args  strings.Builder
	full  string
}

// NewToolCallAccumulator creates a new accumulator for streaming tool calls.
func NewToolCallAccumulator() *ToolCallAccumulator {
	return &ToolCallAccumulator{calls: map[int]*toolCallState{}}
}

// Apply ingests a single delta. It returns true if the delta affected tool state.
func (a *ToolCallAccumulator) Apply(delta NormalizedDelta) bool {
	switch delta.Type {
	case DeltaToolCallBegin, DeltaToolCallArgumentsDelta, DeltaToolCallDone:
		// continue
	default:
		return false
	}

	call := a.ensure(delta.ToolCallIndex)
	switch delta.Type {
	case DeltaToolCallBegin:
		if call.id == "" {
			call.id = delta.ToolCallID
		}
		if call.name == "" {
			call.name = delta.ToolCallName
		}
		if call.sig == "" {
			call.sig = delta.ToolCallSignature
		}
	case DeltaToolCallArgumentsDelta:
		call.args.WriteString(delta.ArgumentsDelta)
	case DeltaToolCallDone:
		if call.id == "" {
			call.id = delta.ToolCallID
		}
		if call.name == "" {
			call.name = delta.ToolCallName
		}
		if call.sig == "" {
			call.sig = delta.ToolCallSignature
		}
		if delta.ArgumentsFull != "" {
			call.full = delta.ArgumentsFull
		}
	}

	return true
}

// CompleteCalls returns fully assembled tool calls in their original order.
// Any missing IDs are filled with stable synthetic IDs.
func (a *ToolCallAccumulator) CompleteCalls() []StreamToolCall {
	if len(a.order) == 0 {
		return nil
	}
	out := make([]StreamToolCall, 0, len(a.order))
	for _, idx := range a.order {
		call := a.calls[idx]
		if call == nil {
			continue
		}
		id := call.id
		if id == "" {
			id = fmt.Sprintf("tool-%d", call.index)
		}
		full := call.full
		if full == "" {
			full = call.args.String()
		}
		out = append(out, StreamToolCall{
			ID:               id,
			Name:             call.name,
			Arguments:        json.RawMessage(full),
			ThoughtSignature: call.sig,
		})
	}
	return out
}

func (a *ToolCallAccumulator) ensure(index int) *toolCallState {
	call := a.calls[index]
	if call != nil {
		return call
	}
	call = &toolCallState{index: index}
	a.calls[index] = call
	a.order = append(a.order, index)
	return call
}
