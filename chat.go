package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ChatTargetFriend = "friend"
	ChatTargetGroup  = "group"
)

type ChatTarget struct {
	Kind      string
	SourceID  string
	UserID    string
	RobotJSON string
}

type ChatService struct {
	configPath string
	sessions   *SessionStore
	client     chatCompleter
	log        func(string)
}

var pluginRuntime = struct {
	sync.RWMutex
	chat *ChatService
	http *HTTPService
}{}

func ParseAIInput(message, prefix string) (string, bool) {
	message = strings.TrimSpace(message)
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return strings.TrimSpace(message), strings.TrimSpace(message) != ""
	}
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	input := strings.TrimSpace(strings.TrimPrefix(message, prefix))
	return input, input != ""
}

func NewChatService(configPath string, sessions *SessionStore, client chatCompleter) *ChatService {
	return &ChatService{configPath: configPath, sessions: sessions, client: client}
}

func (s *ChatService) Handle(ctx context.Context, target ChatTarget, message string, send func(MarkdownMessage) error) bool {
	if s == nil || s.client == nil || s.sessions == nil || send == nil {
		return false
	}
	cfg, err := LoadAIConfig(s.configPath)
	if err != nil {
		s.logError("读取AI配置失败: " + err.Error())
		return false
	}
	if !target.enabled(cfg) {
		return false
	}
	input := ""
	ok := false
	switch target.Kind {
	case ChatTargetFriend:
		input, ok = ParseAIInput(message, "")
	case ChatTargetGroup:
		if cfg.PublicTriggerMode == PublicTriggerMention {
			input, ok = ParseMentionAIInput(target.RobotJSON, message)
		} else {
			input, ok = ParseAIInput(message, cfg.PublicPrefix)
		}
	default:
		return false
	}
	if !ok {
		return false
	}
	sessionKey := SessionKey(target.UserID)
	history, err := s.sessions.Load(sessionKey)
	if err != nil {
		s.logError("读取AI会话失败: " + err.Error())
		return false
	}
	messages, err := BuildMessagesForTurn(ctx, s.client, cfg, history, input)
	if err != nil {
		s.logError("构造AI消息失败: " + err.Error())
		return false
	}
	reply, err := s.client.Chat(ctx, cfg.Model, messages)
	if err != nil {
		reply = ChatMessage{Role: "assistant", Content: friendlyAIError(err)}
	}
	if strings.TrimSpace(reply.Content) == "" {
		reply = ChatMessage{Role: "assistant", Content: friendlyAIError(ErrAIUnavailable)}
	}
	if err := send(MarkdownMessage{Native: reply.Content}); err != nil {
		s.logError("发送AI回复失败: " + err.Error())
		return true
	}
	messages = append(messages, reply)
	if err := s.sessions.Save(sessionKey, messages); err != nil {
		s.logError("保存AI会话失败: " + err.Error())
	}
	return true
}

func ParseMentionAIInput(robotJSON, message string) (string, bool) {
	mentioned, ids := mentionedSelf(robotJSON)
	if !mentioned {
		return "", false
	}
	input := strings.TrimSpace(message)
	for _, id := range ids {
		input = strings.ReplaceAll(input, "<@"+id+">", "")
		input = strings.ReplaceAll(input, "<@!"+id+">", "")
	}
	input = strings.TrimSpace(input)
	return input, input != ""
}

func mentionedSelf(robotJSON string) (bool, []string) {
	var ctx struct {
		Raw json.RawMessage `json:"raw"`
	}
	if err := json.Unmarshal([]byte(robotJSON), &ctx); err != nil {
		return false, nil
	}
	raw := ctx.Raw
	if len(raw) == 0 {
		raw = []byte(robotJSON)
	} else {
		var rawText string
		if err := json.Unmarshal(raw, &rawText); err == nil {
			raw = []byte(rawText)
		}
	}
	var event struct {
		D struct {
			Mentions []struct {
				ID    string `json:"id"`
				IsYou bool   `json:"is_you"`
			} `json:"mentions"`
		} `json:"d"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return false, nil
	}
	var ids []string
	for _, mention := range event.D.Mentions {
		if mention.IsYou {
			if mention.ID != "" {
				ids = append(ids, mention.ID)
			}
		}
	}
	return len(ids) > 0, ids
}

func (s *ChatService) logError(text string) {
	if s != nil && s.log != nil {
		s.log(text)
	}
}

func (target ChatTarget) enabled(cfg AIConfig) bool {
	switch target.Kind {
	case ChatTargetFriend:
		return cfg.EnableFriend
	case ChatTargetGroup:
		return cfg.EnableGroup
	default:
		return false
	}
}

func friendlyAIError(err error) string {
	if errors.Is(err, ErrModelRequired) {
		return ErrModelRequired.Error()
	}
	return ErrAIUnavailable.Error()
}

func currentChatService() *ChatService {
	pluginRuntime.RLock()
	defer pluginRuntime.RUnlock()
	return pluginRuntime.chat
}

func currentHTTPService() *HTTPService {
	pluginRuntime.RLock()
	defer pluginRuntime.RUnlock()
	return pluginRuntime.http
}

func ensurePluginServices(dataDir string, logger func(string)) {
	pluginRuntime.RLock()
	ready := pluginRuntime.chat != nil && pluginRuntime.http != nil
	pluginRuntime.RUnlock()
	if ready {
		return
	}
	InitializePluginServices(dataDir, logger)
}

func pluginDataDir() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Dir(executable)), nil
}

func InitializePluginServices(dataDir string, logger func(string)) {
	configPath := filepath.Join(dataDir, "config.json")
	cfg, err := LoadAIConfig(configPath)
	if err == nil {
		_ = SaveAIConfig(configPath, cfg)
	}
	client := NewAIClientWithProxy("", cfg.ProxyAddress)
	chat := NewChatService(configPath, NewSessionStore(filepath.Join(dataDir, "sessions")), client)
	chat.log = logger
	pluginRuntime.Lock()
	pluginRuntime.chat = chat
	pluginRuntime.http = NewHTTPService(configPath, client)
	pluginRuntime.Unlock()
}

func StopPluginHTTPService() {
	service := currentHTTPService()
	if service == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = service.Stop(ctx)
}
