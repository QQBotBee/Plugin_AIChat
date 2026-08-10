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
	defaultConversationLimit = 80
	minConversationLimit     = 4
)

const defaultSystemPrompt = `你的名字：小猪猪
回复要求：始终以调皮可爱的小猪猪口吻进行对话。绝对禁止使用任何Markdown语法。这意味着你的回复中严禁出现双星号（用于加粗）、井号（用于标题）、反引号（用于代码块）等任何格式化符号。仅允许使用纯文字、emoji表情、特殊符号（可用于列表等特殊场景）。如果需要分段，请直接使用换行符，不要使用任何引导符。这是一个硬性约束，任何Markdown符号的出现都视为任务失败。`

type AIConfig struct {
	Port              int    `json:"port"`
	Model             string `json:"model"`
	SystemPrompt      string `json:"system_prompt"`
	ConversationLimit int    `json:"conversation_limit"`
	PublicPrefix      string `json:"public_prefix"`
	EnableFriend      bool   `json:"enable_friend"`
	EnableGroup       bool   `json:"enable_group"`
	EnableChannel     bool   `json:"enable_channel"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		Port:              defaultHTTPPort,
		SystemPrompt:      defaultSystemPrompt,
		ConversationLimit: defaultConversationLimit,
		PublicPrefix:      "#",
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
	cfg.PublicPrefix = strings.TrimSpace(cfg.PublicPrefix)
	if cfg.PublicPrefix == "" {
		cfg.PublicPrefix = "#"
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
