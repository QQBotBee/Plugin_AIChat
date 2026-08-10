package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCompleter struct {
	calls    int
	messages []ChatMessage
	reply    ChatMessage
	err      error
}

func (f *fakeCompleter) Chat(ctx context.Context, model string, messages []ChatMessage) (ChatMessage, error) {
	f.calls++
	f.messages = append([]ChatMessage(nil), messages...)
	if f.err != nil {
		return ChatMessage{}, f.err
	}
	return f.reply, nil
}

func TestSessionStoreRoundTripUsesSafeFilename(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	store := NewSessionStore(root)
	key := SessionKey("group", "group/with/slash", "user:42")
	want := []ChatMessage{
		{Role: "system", Content: "系统"},
		{Role: "user", Content: "你好"},
	}

	if err := store.Save(key, want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	got, err := store.Load(key)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(got) != len(want) || got[1].Content != "你好" {
		t.Fatalf("loaded = %#v, want %#v", got, want)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "*")); len(matches) != 1 || strings.Contains(matches[0], "group/with/slash") {
		t.Fatalf("session files = %#v, want one encoded filename", matches)
	}
}

func TestSessionKeyIncludesKindSourceAndUser(t *testing.T) {
	got := SessionKey("channel", "sub123", "user456")
	if got != "channel/sub123/user456" {
		t.Fatalf("SessionKey = %q, want channel/sub123/user456", got)
	}
}

func TestBuildMessagesForTurnStartsWithSystemPrompt(t *testing.T) {
	cfg := NormalizeAIConfig(AIConfig{Model: "alpha-free", SystemPrompt: "系统预设", ConversationLimit: 42})

	got, err := BuildMessagesForTurn(context.Background(), nil, cfg, nil, "你好")
	if err != nil {
		t.Fatalf("BuildMessagesForTurn returned error: %v", err)
	}
	want := []ChatMessage{{Role: "system", Content: "系统预设"}, {Role: "user", Content: "你好"}}
	if len(got) != len(want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("messages = %#v, want %#v", got, want)
		}
	}
}

func TestBuildMessagesForTurnCompressesWhenLimitReached(t *testing.T) {
	cfg := NormalizeAIConfig(AIConfig{Model: "alpha-free", SystemPrompt: "系统预设", ConversationLimit: 4})
	completer := &fakeCompleter{reply: ChatMessage{Role: "assistant", Content: "[记忆概述：用户喜欢Go]"}}
	history := []ChatMessage{
		{Role: "system", Content: "系统预设"},
		{Role: "user", Content: "第一句"},
		{Role: "assistant", Content: "回复一"},
		{Role: "user", Content: "第二句"},
	}

	got, err := BuildMessagesForTurn(context.Background(), completer, cfg, history, "新问题")
	if err != nil {
		t.Fatalf("BuildMessagesForTurn returned error: %v", err)
	}
	if completer.calls != 1 {
		t.Fatalf("compress calls = %d, want 1", completer.calls)
	}
	if len(completer.messages) != len(history)+1 {
		t.Fatalf("compress messages len = %d, want %d", len(completer.messages), len(history)+1)
	}
	if !strings.Contains(completer.messages[len(completer.messages)-1].Content, "[记忆概述：内容]") {
		t.Fatalf("compress prompt = %q, want strict memory format", completer.messages[len(completer.messages)-1].Content)
	}
	want := []ChatMessage{
		{Role: "system", Content: "系统预设"},
		{Role: "system", Content: "[记忆概述：用户喜欢Go]"},
		{Role: "user", Content: "新问题"},
	}
	if len(got) != len(want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("messages = %#v, want %#v", got, want)
		}
	}
}
