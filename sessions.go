package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const memoryCompressionPrompt = "简要概括一下咱们两个的聊天记录，用于压缩记忆的目的，列出我的特征、关键事件、习惯爱好、特长等关键内容，严格按照以下格式输出，禁止有任何多余内容：[记忆概述：内容]"

type chatCompleter interface {
	Chat(ctx context.Context, model string, messages []ChatMessage) (ChatMessage, error)
	ChatMultimodal(ctx context.Context, model string, messages []ChatRequestMessage) (ChatMessage, error)
}

type SessionStore struct {
	root string
}

func NewSessionStore(root string) *SessionStore {
	return &SessionStore{root: root}
}

func (s *SessionStore) Load(key string) ([]ChatMessage, error) {
	data, err := os.ReadFile(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var messages []ChatMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *SessionStore) Save(key string, messages []ChatMessage) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(s.path(key), data, 0o644)
}

func (s *SessionStore) path(key string) string {
	name := sessionFilename(key)
	return filepath.Join(s.root, name)
}

func SessionKey(userID string) string {
	return strings.TrimSpace(userID)
}

func sessionFilename(userID string) string {
	name := strings.TrimSpace(userID)
	replacer := strings.NewReplacer(
		`<`, "_",
		`>`, "_",
		`:`, "_",
		`"`, "_",
		`/`, "_",
		`\`, "_",
		`|`, "_",
		`?`, "_",
		`*`, "_",
	)
	name = replacer.Replace(name)
	if name == "" {
		name = "unknown"
	}
	return name + ".json"
}

func BuildMessagesForTurn(ctx context.Context, client chatCompleter, cfg AIConfig, history []ChatMessage, userInput string) ([]ChatMessage, error) {
	cfg = NormalizeAIConfig(cfg)
	userInput = strings.TrimSpace(userInput)
	if len(history) == 0 {
		return []ChatMessage{
			{Role: "system", Content: cfg.SystemPrompt},
			{Role: "user", Content: userInput},
		}, nil
	}
	if len(history) >= cfg.ConversationLimit && client != nil {
		summaryMessages := append([]ChatMessage(nil), history...)
		summaryMessages = append(summaryMessages, ChatMessage{Role: "user", Content: memoryCompressionPrompt})
		summary, err := client.Chat(ctx, cfg.Model, summaryMessages)
		if err == nil && strings.TrimSpace(summary.Content) != "" {
			return []ChatMessage{
				{Role: "system", Content: cfg.SystemPrompt},
				{Role: "system", Content: strings.TrimSpace(summary.Content)},
				{Role: "user", Content: userInput},
			}, nil
		}
	}
	messages := append([]ChatMessage(nil), history...)
	messages = append(messages, ChatMessage{Role: "user", Content: userInput})
	return messages, nil
}
