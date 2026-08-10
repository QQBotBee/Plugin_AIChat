package main

import (
	"context"
	"path/filepath"
	"testing"
)

func TestParseAIInputRequiresHashPrefix(t *testing.T) {
	if _, ok := ParseAIInput("你好"); ok {
		t.Fatal("ParseAIInput accepted message without # prefix")
	}
}

func TestParseAIInputTrimsAfterPrefix(t *testing.T) {
	got, ok := ParseAIInput("#  你好  ")
	if !ok {
		t.Fatal("ParseAIInput rejected prefixed message")
	}
	if got != "你好" {
		t.Fatalf("input = %q, want 你好", got)
	}
}

func TestChatServiceSkipsDisabledWorkArea(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := DefaultAIConfig()
	cfg.EnableGroup = false
	cfg.Model = "alpha-free"
	if err := SaveAIConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	client := &fakeCompleter{reply: ChatMessage{Role: "assistant", Content: "不应该调用"}}
	service := NewChatService(configPath, NewSessionStore(filepath.Join(t.TempDir(), "sessions")), client)

	handled := service.Handle(context.Background(), ChatTarget{Kind: ChatTargetGroup, SourceID: "group1", UserID: "user1"}, "#你好", func(message MarkdownMessage) error {
		t.Fatal("send should not be called for disabled group")
		return nil
	})
	if handled {
		t.Fatal("Handle returned true for disabled group")
	}
	if client.calls != 0 {
		t.Fatalf("client calls = %d, want 0", client.calls)
	}
}

func TestChatServiceSendsMarkdownNativeReply(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	cfg := DefaultAIConfig()
	cfg.Model = "alpha-free"
	if err := SaveAIConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	client := &fakeCompleter{reply: ChatMessage{Role: "assistant", Content: "## 你好\n这是回复"}}
	service := NewChatService(configPath, NewSessionStore(filepath.Join(dir, "sessions")), client)

	var sent []MarkdownMessage
	handled := service.Handle(context.Background(), ChatTarget{Kind: ChatTargetFriend, SourceID: "friend1", UserID: "friend1"}, "#你好", func(message MarkdownMessage) error {
		sent = append(sent, message)
		return nil
	})
	if !handled {
		t.Fatal("Handle returned false for prefixed friend message")
	}
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if sent[0].Native != "## 你好\n这是回复" {
		t.Fatalf("Native = %q, want AI reply", sent[0].Native)
	}
	if sent[0].TemplateID != "" || sent[0].TemplateIndex != 0 {
		t.Fatalf("sent markdown = %#v, want custom native markdown only", sent[0])
	}
}

func TestInitializePluginServicesCreatesRuntimeServices(t *testing.T) {
	InitializePluginServices(t.TempDir(), nil)
	t.Cleanup(func() {
		StopPluginHTTPService()
		pluginRuntime.Lock()
		pluginRuntime.chat = nil
		pluginRuntime.http = nil
		pluginRuntime.Unlock()
	})

	if currentChatService() == nil {
		t.Fatal("currentChatService() = nil, want initialized service")
	}
	if currentHTTPService() == nil {
		t.Fatal("currentHTTPService() = nil, want initialized service")
	}
	StopPluginHTTPService()
}
