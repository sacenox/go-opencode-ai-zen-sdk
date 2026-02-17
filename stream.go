package zen

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type StreamEvent struct {
	Event string
	Data  json.RawMessage
	Raw   string
}

type Stream struct {
	Events <-chan StreamEvent
	Err    error
	Close  func() error
}

func (c *Client) startStream(ctx context.Context, method, path string, body []byte) (*Stream, error) {
	url := joinURL(c.cfg.BaseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, newAPIError(resp.StatusCode, resp.Header, payload)
	}

	events := make(chan StreamEvent)
	stream := &Stream{
		Events: events,
		Close:  resp.Body.Close,
	}

	go func() {
		defer close(events)
		reader := bufio.NewReader(resp.Body)
		var eventName string
		var dataBuf bytes.Buffer

		flush := func() bool {
			if dataBuf.Len() == 0 {
				eventName = ""
				return false
			}
			raw := strings.TrimSuffix(dataBuf.String(), "\n")
			dataBuf.Reset()
			name := eventName
			eventName = ""

			if raw == "[DONE]" {
				return true
			}

			events <- StreamEvent{
				Event: name,
				Data:  json.RawMessage(raw),
				Raw:   raw,
			}
			return false
		}

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					flush()
				} else {
					stream.Err = err
				}
				return
			}

			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if done := flush(); done {
					return
				}
				continue
			}

			if strings.HasPrefix(line, ":") {
				continue
			}

			if strings.HasPrefix(line, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				dataBuf.WriteString(data)
				dataBuf.WriteByte('\n')
			}
		}
	}()

	return stream, nil
}
