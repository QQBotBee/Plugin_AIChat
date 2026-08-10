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

const (
	PublicTriggerPrefix  = "prefix"
	PublicTriggerMention = "mention"
)

const defaultSystemPrompt = "你的名字：小猪猪。回复要求：始终以调皮可爱的小猪猪口吻进行对话。"

type AIConfig struct {
	Port              int    `json:"port"`
	Model             string `json:"model"`
	SystemPrompt      string `json:"system_prompt"`
	ConversationLimit int    `json:"conversation_limit"`
	ProxyAddress      string `json:"proxy_address"`
	PublicPrefix      string `json:"public_prefix"`
	PublicTriggerMode string `json:"public_trigger_mode"`
	EnableFriend      bool   `json:"enable_friend"`
	EnableGroup       bool   `json:"enable_group"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		Port:              defaultHTTPPort,
		SystemPrompt:      defaultSystemPrompt,
		ConversationLimit: defaultConversationLimit,
		PublicPrefix:      "#",
		PublicTriggerMode: PublicTriggerPrefix,
		EnableFriend:      true,
		EnableGroup:       true,
	}
}

func NormalizeAIConfig(cfg AIConfig) AIConfig {
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.SystemPrompt = strings.TrimSpace(cfg.SystemPrompt)
	cfg.ProxyAddress = strings.TrimSpace(cfg.ProxyAddress)
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
	cfg.PublicTriggerMode = strings.TrimSpace(cfg.PublicTriggerMode)
	if cfg.PublicTriggerMode != PublicTriggerMention {
		cfg.PublicTriggerMode = PublicTriggerPrefix
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
