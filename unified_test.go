package zen

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type requestCapture struct {
	path string
	body map[string]any
}

func TestUnifiedCreateNormalizedRouting(t *testing.T) {
	var mu sync.Mutex
	var captures []requestCapture

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		var body map[string]any
		_ = json.Unmarshal(payload, &body)

		mu.Lock()
		captures = append(captures, requestCapture{path: r.URL.Path, body: body})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{APIKey: "key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("client error: %v", err)
	}

	ctx := context.Background()

	_, err = client.UnifiedCreateNormalized(ctx, NormalizedRequest{
		Model:  "gpt-5.2-codex",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("responses request error: %v", err)
	}

	_, err = client.UnifiedCreateNormalized(ctx, NormalizedRequest{
		Model:  "claude-sonnet-4-6",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("messages request error: %v", err)
	}

	_, err = client.UnifiedCreateNormalized(ctx, NormalizedRequest{
		Model:  "gemini-3-pro",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("gemini request error: %v", err)
	}

	_, err = client.UnifiedCreateNormalized(ctx, NormalizedRequest{
		Model:  "glm-5",
		System: "system",
		Messages: []NormalizedMessage{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("chat completion request error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captures) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(captures))
	}

	if captures[0].path != "/responses" {
		t.Fatalf("expected /responses, got %s", captures[0].path)
	}
	if _, ok := captures[0].body["input"]; !ok {
		t.Fatalf("responses payload missing input")
	}

	if captures[1].path != "/messages" {
		t.Fatalf("expected /messages, got %s", captures[1].path)
	}
	if _, ok := captures[1].body["messages"]; !ok {
		t.Fatalf("messages payload missing messages")
	}

	if captures[2].path != "/models/gemini-3-pro" {
		t.Fatalf("expected /models/gemini-3-pro, got %s", captures[2].path)
	}
	if _, ok := captures[2].body["contents"]; !ok {
		t.Fatalf("gemini payload missing contents")
	}

	if captures[3].path != "/chat/completions" {
		t.Fatalf("expected /chat/completions, got %s", captures[3].path)
	}
	if _, ok := captures[3].body["messages"]; !ok {
		t.Fatalf("chat payload missing messages")
	}
}
