package zen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message\n"))
		_, _ = w.Write([]byte("data: {\"a\":1}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	stream, err := client.startStream(context.Background(), EndpointResponses, "POST", "/responses", []byte("{}"))
	if err != nil {
		t.Fatalf("startStream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for ev := range stream.Events {
		events = append(events, ev)
	}

	if stream.Err != nil {
		t.Fatalf("stream err: %v", stream.Err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != "message" {
		t.Fatalf("event name mismatch: %s", events[0].Event)
	}
	if !strings.Contains(string(events[0].Data), "\"a\":1") {
		t.Fatalf("event data mismatch: %s", string(events[0].Data))
	}
}

// TestDefaultClientHasNoBodyTimeout asserts that the http.Client constructed
// by NewClient has Timeout == 0 (unlimited body read) and that its transport
// has no ResponseHeaderTimeout by default (to avoid aborting slow-starting
// streaming responses).
func TestDefaultClientHasNoBodyTimeout(t *testing.T) {
	c, err := NewClient(Config{APIKey: "key"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if c.httpClient.Timeout != 0 {
		t.Fatalf("expected http.Client.Timeout == 0 (unlimited), got %v", c.httpClient.Timeout)
	}

	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.httpClient.Transport)
	}

	if transport.ResponseHeaderTimeout != 0 {
		t.Fatalf("expected ResponseHeaderTimeout == 0, got %v", transport.ResponseHeaderTimeout)
	}
}

// TestSlowStreamNotKilledByClientTimeout verifies that a streaming response
// whose events are separated by pauses longer than the old 60-second default
// (simulated here with a short delay to keep the test fast) is fully received
// without error.
func TestSlowStreamNotKilledByClientTimeout(t *testing.T) {
	const delay = 200 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for i := 0; i < 3; i++ {
			_, _ = w.Write([]byte("data: {\"i\":" + string(rune('0'+i)) + "}\n\n"))
			flusher.Flush()
			time.Sleep(delay)
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	// No Config.Timeout is set (defaults to 0 = unlimited).  The test
	// confirms that the default client configuration does not kill a stream
	// body that is actively delivering events over an extended duration.
	client, err := NewClient(Config{
		APIKey:  "key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	stream, err := client.startStream(context.Background(), EndpointResponses, "POST", "/responses", []byte("{}"))
	if err != nil {
		t.Fatalf("startStream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for ev := range stream.Events {
		events = append(events, ev)
	}

	if stream.Err != nil {
		t.Fatalf("stream terminated with error: %v", stream.Err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}
