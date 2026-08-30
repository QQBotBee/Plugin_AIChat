package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultHTTPPort                = 8765
	defaultConversationLimit       = 80
	minConversationLimit           = 4
	defaultOpenAICompatibleBaseURL = "https://api.openai.com/v1"
)

const (
	ProviderOpenCode         = "opencode"
	ProviderOpenAICompatible = "openai_compatible"
	PublicTriggerPrefix      = "prefix"
	PublicTriggerMention     = "mention"
)

const defaultSystemPrompt = "你的名字：小猪猪。回复要求：始终以调皮可爱的小猪猪口吻进行对话。绝对禁止使用任何Markdown语法。"

type AIConfig struct {
	ProviderType      string `json:"provider_type"`
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	Port              int    `json:"port"`
	Model             string `json:"model"`
	SystemPrompt      string `json:"system_prompt"`
	ConversationLimit int    `json:"conversation_limit"`
	ProxyAddress      string `json:"proxy_address"`
	PublicPrefix      string `json:"public_prefix"`
	PublicTriggerMode string `json:"public_trigger_mode"`
	EnableFriend      bool   `json:"enable_friend"`
	EnableGroup       bool   `json:"enable_group"`
	EnableChannel     bool   `json:"enable_channel"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		ProviderType:      ProviderOpenCode,
		BaseURL:           defaultAIBaseURL,
		Port:              defaultHTTPPort,
		SystemPrompt:      defaultSystemPrompt,
		ConversationLimit: defaultConversationLimit,
		PublicPrefix:      "#",
		PublicTriggerMode: PublicTriggerPrefix,
		EnableFriend:      true,
		EnableGroup:       true,
		EnableChannel:     true,
	}
}

func NormalizeAIConfig(cfg AIConfig) AIConfig {
	cfg.ProviderType = strings.ToLower(strings.TrimSpace(cfg.ProviderType))
	switch cfg.ProviderType {
	case "", ProviderOpenCode:
		cfg.ProviderType = ProviderOpenCode
		cfg.BaseURL = defaultAIBaseURL
	case ProviderOpenAICompatible:
		cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
		if cfg.BaseURL == "" {
			cfg.BaseURL = defaultOpenAICompatibleBaseURL
		}
	default:
		cfg.ProviderType = ProviderOpenCode
		cfg.BaseURL = defaultAIBaseURL
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.SystemPrompt = strings.TrimSpace(cfg.SystemPrompt)
	cfg.ProxyAddress = strings.TrimSpace(cfg.ProxyAddress)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
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
