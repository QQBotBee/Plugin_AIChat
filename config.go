package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultHTTPPort          = 8765
	defaultConversationLimit = 42
	minConversationLimit     = 4
)

const defaultSystemPrompt = "你是一个简易且实用的AI智能聊天助手，请用简洁、准确、自然的中文回答用户。"

type AIConfig struct {
	Port              int    `json:"port"`
	Model             string `json:"model"`
	SystemPrompt      string `json:"system_prompt"`
	ConversationLimit int    `json:"conversation_limit"`
	EnableFriend      bool   `json:"enable_friend"`
	EnableGroup       bool   `json:"enable_group"`
	EnableChannel     bool   `json:"enable_channel"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		Port:              defaultHTTPPort,
		SystemPrompt:      defaultSystemPrompt,
		ConversationLimit: defaultConversationLimit,
		EnableFriend:      true,
		EnableGroup:       true,
		EnableChannel:     true,
	}
}

func NormalizeAIConfig(cfg AIConfig) AIConfig {
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.SystemPrompt = strings.TrimSpace(cfg.SystemPrompt)
	if cfg.Port <= 0 || cfg.Port > 65535 {
		cfg.Port = defaultHTTPPort
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = defaultSystemPrompt
	}
	if cfg.ConversationLimit <= 0 {
		cfg.ConversationLimit = defaultConversationLimit
	}
	if cfg.ConversationLimit < minConversationLimit {
		cfg.ConversationLimit = minConversationLimit
	}
	return cfg
}

func LoadAIConfig(path string) (AIConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultAIConfig(), nil
	}
	if err != nil {
		return AIConfig{}, err
	}
	var cfg AIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AIConfig{}, err
	}
	return NormalizeAIConfig(cfg), nil
}

func SaveAIConfig(path string, cfg AIConfig) error {
	cfg = NormalizeAIConfig(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
