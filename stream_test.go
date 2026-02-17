package zen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	stream, err := client.startStream(context.Background(), "POST", "/responses", []byte("{}"))
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
