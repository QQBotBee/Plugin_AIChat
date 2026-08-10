package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIClientListFreeModelsFiltersFreeIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("path = %q, want /models", r.URL.Path)
		}
		assertOpenCodeHeaders(t, r)
		_, _ = w.Write([]byte(`{"data":[{"id":"alpha-free"},{"id":"beta-paid"},{"id":"gamma-free"}]}`))
	}))
	defer server.Close()

	models, err := NewAIClient(server.URL, server.Client()).ListFreeModels(context.Background())
	if err != nil {
		t.Fatalf("ListFreeModels returned error: %v", err)
	}
	want := []string{"alpha-free", "gamma-free"}
	if len(models) != len(want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Fatalf("models = %#v, want %#v", models, want)
		}
	}
}

func TestAIClientChatPostsOpenAICompatiblePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		assertOpenCodeHeaders(t, r)
		var body struct {
			Model            string        `json:"model"`
			Stream           bool          `json:"stream"`
			IncludeReasoning bool          `json:"include_reasoning"`
			Messages         []ChatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "alpha-free" {
			t.Fatalf("model = %q, want alpha-free", body.Model)
		}
		if body.Stream {
			t.Fatal("stream = true, want false")
		}
		if body.IncludeReasoning {
			t.Fatal("include_reasoning = true, want false")
		}
		if len(body.Messages) != 1 || body.Messages[0] != (ChatMessage{Role: "user", Content: "你好"}) {
			t.Fatalf("messages = %#v, want one user message", body.Messages)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你好，我在"}}]}`))
	}))
	defer server.Close()

	reply, err := NewAIClient(server.URL, server.Client()).Chat(context.Background(), "alpha-free", []ChatMessage{{Role: "user", Content: "你好"}})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if reply != (ChatMessage{Role: "assistant", Content: "你好，我在"}) {
		t.Fatalf("reply = %#v, want assistant content", reply)
	}
}

func TestAIClientChatRejectsEmptyModel(t *testing.T) {
	_, err := NewAIClient("http://127.0.0.1", http.DefaultClient).Chat(context.Background(), "  ", []ChatMessage{{Role: "user", Content: "hi"}})
	if !errors.Is(err, ErrModelRequired) {
		t.Fatalf("err = %v, want ErrModelRequired", err)
	}
}

func TestAIClientChatReturnsUnavailableOnEmptyChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""}}]}`))
	}))
	defer server.Close()

	_, err := NewAIClient(server.URL, server.Client()).Chat(context.Background(), "alpha-free", []ChatMessage{{Role: "user", Content: "hi"}})
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("err = %v, want ErrAIUnavailable", err)
	}
}

func assertOpenCodeHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	checks := map[string]string{
		"Authorization":      "Bearer public",
		"x-opencode-project": "proj_example",
		"x-opencode-session": "sess_example",
		"x-opencode-request": "msg_example",
		"x-opencode-client":  "cli",
	}
	for key, want := range checks {
		if got := r.Header.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}
