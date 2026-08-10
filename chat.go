package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ChatTargetFriend  = "friend"
	ChatTargetGroup   = "group"
	ChatTargetChannel = "channel"
)

type ChatTarget struct {
	Kind     string
	SourceID string
	UserID   string
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

func ParseAIInput(message string) (string, bool) {
	if !strings.HasPrefix(message, "#") {
		return "", false
	}
	input := strings.TrimSpace(strings.TrimPrefix(message, "#"))
	return input, input != ""
}

func NewChatService(configPath string, sessions *SessionStore, client chatCompleter) *ChatService {
	return &ChatService{configPath: configPath, sessions: sessions, client: client}
}

func (s *ChatService) Handle(ctx context.Context, target ChatTarget, message string, send func(MarkdownMessage) error) bool {
	input, ok := ParseAIInput(message)
	if !ok || s == nil || s.client == nil || s.sessions == nil || send == nil {
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
	sessionKey := SessionKey(target.Kind, target.SourceID, target.UserID)
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
	case ChatTargetChannel:
		return cfg.EnableChannel
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

func InitializePluginServices(dataDir string, logger func(string)) {
	configPath := filepath.Join(dataDir, "config.json")
	cfg, err := LoadAIConfig(configPath)
	if err == nil {
		_ = SaveAIConfig(configPath, cfg)
	}
	client := NewAIClient("", nil)
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
