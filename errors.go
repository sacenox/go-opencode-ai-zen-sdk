package zen

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type APIError struct {
	StatusCode int
	RequestID  string
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("zen: request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("zen: request failed with status %d: %s", e.StatusCode, e.Message)
}

type apiErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Message string `json:"message"`
}

func newAPIError(status int, header http.Header, body []byte) *APIError {
	reqID := header.Get("x-request-id")
	if reqID == "" {
		reqID = header.Get("request-id")
	}

	msg := ""
	var env apiErrorEnvelope
	if err := json.Unmarshal(body, &env); err == nil {
		if env.Error.Message != "" {
			msg = env.Error.Message
		} else if env.Message != "" {
			msg = env.Message
		}
	}

	return &APIError{
		StatusCode: status,
		RequestID:  reqID,
		Message:    msg,
		Body:       body,
	}
}
