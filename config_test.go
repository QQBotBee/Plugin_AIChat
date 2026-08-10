package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultAIConfigHasExpectedRuntimeDefaults(t *testing.T) {
	cfg := DefaultAIConfig()

	if cfg.Port != 8765 {
		t.Fatalf("Port = %d, want 8765", cfg.Port)
	}
	if cfg.ConversationLimit != 80 {
		t.Fatalf("ConversationLimit = %d, want 80", cfg.ConversationLimit)
	}
	if cfg.Model != "" {
		t.Fatalf("Model = %q, want empty", cfg.Model)
	}
	if cfg.SystemPrompt == "" {
		t.Fatal("SystemPrompt is empty")
	}
	if cfg.PublicPrefix != "#" {
		t.Fatalf("PublicPrefix = %q, want #", cfg.PublicPrefix)
	}
	if !cfg.EnableFriend || !cfg.EnableGroup || !cfg.EnableChannel {
		t.Fatalf("work areas = friend:%v group:%v channel:%v, want all enabled", cfg.EnableFriend, cfg.EnableGroup, cfg.EnableChannel)
	}
}

func TestLoadAIConfigMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadAIConfig(filepath.Join(t.TempDir(), "missing", "config.json"))
	if err != nil {
		t.Fatalf("LoadAIConfig returned error: %v", err)
	}

	want := DefaultAIConfig()
	if cfg != want {
		t.Fatalf("LoadAIConfig missing = %+v, want %+v", cfg, want)
	}
}

func TestSaveAndLoadAIConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := AIConfig{
		Port:              9090,
		Model:             "model-free",
		SystemPrompt:      "你是一个测试助手",
		ConversationLimit: 10,
		PublicPrefix:      "!",
		EnableFriend:      true,
		EnableGroup:       false,
		EnableChannel:     true,
	}

	if err := SaveAIConfig(path, want); err != nil {
		t.Fatalf("SaveAIConfig returned error: %v", err)
	}
	got, err := LoadAIConfig(path)
	if err != nil {
		t.Fatalf("LoadAIConfig returned error: %v", err)
	}
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestNormalizeAIConfigClampsInvalidPortAndLimit(t *testing.T) {
	cfg := NormalizeAIConfig(AIConfig{
		Port:              70000,
		Model:             "  model-free  ",
		SystemPrompt:      "",
		ConversationLimit: 1,
		PublicPrefix:      "  !  ",
	})

	if cfg.Port != 8765 {
		t.Fatalf("Port = %d, want 8765", cfg.Port)
	}
	if cfg.ConversationLimit != 4 {
		t.Fatalf("ConversationLimit = %d, want 4", cfg.ConversationLimit)
	}
	if cfg.Model != "model-free" {
		t.Fatalf("Model = %q, want trimmed model-free", cfg.Model)
	}
	if cfg.SystemPrompt == "" {
		t.Fatal("SystemPrompt is empty after normalize")
	}
	if cfg.PublicPrefix != "!" {
		t.Fatalf("PublicPrefix = %q, want !", cfg.PublicPrefix)
	}
}

func TestDefaultAIConfigUsesPiggySystemPrompt(t *testing.T) {
	cfg := DefaultAIConfig()

	if want := "你的名字：小猪猪"; !strings.Contains(cfg.SystemPrompt, want) {
		t.Fatalf("SystemPrompt = %q, want contain %q", cfg.SystemPrompt, want)
	}
	if want := "绝对禁止使用任何Markdown语法"; !strings.Contains(cfg.SystemPrompt, want) {
		t.Fatalf("SystemPrompt = %q, want contain %q", cfg.SystemPrompt, want)
	}
}
