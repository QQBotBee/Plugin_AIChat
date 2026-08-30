package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultAIBaseURL = "https://opencode.ai/zen/v1"

var (
	ErrModelRequired = errors.New("请先设置对话大模型")
	ErrAIUnavailable = errors.New("网络异常，请稍后重试……")
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model            string               `json:"model"`
	Stream           bool                 `json:"stream"`
	IncludeReasoning bool                 `json:"include_reasoning"`
	Messages         []ChatRequestMessage `json:"messages"`
}

type AIClient struct {
	mu           sync.RWMutex
	baseURL      string
	apiKey       string
	providerType string
	http         *http.Client
}

func NewAIClient(baseURL string, httpClient *http.Client) *AIClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultAIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &AIClient{baseURL: baseURL, providerType: ProviderOpenCode, http: httpClient}
}

func NewAIClientWithProxy(baseURL, proxyAddress string) *AIClient {
	return NewAIClient(baseURL, newAIHTTPClient(proxyAddress))
}

func NewAIClientFromConfig(cfg AIConfig, httpClient *http.Client) *AIClient {
	cfg = NormalizeAIConfig(cfg)
	if httpClient == nil {
		httpClient = newAIHTTPClient(cfg.ProxyAddress)
	}
	return &AIClient{
		baseURL:      cfg.BaseURL,
		apiKey:       cfg.APIKey,
		providerType: cfg.ProviderType,
		http:         httpClient,
	}
}

func (c *AIClient) ConfigureProxy(proxyAddress string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.http = newAIHTTPClient(proxyAddress)
	c.mu.Unlock()
}

func (c *AIClient) UpdateConfig(cfg AIConfig) {
	if c == nil {
		return
	}
	cfg = NormalizeAIConfig(cfg)
	c.mu.Lock()
	c.baseURL = cfg.BaseURL
	c.apiKey = cfg.APIKey
	c.providerType = cfg.ProviderType
	c.mu.Unlock()
	c.ConfigureProxy(cfg.ProxyAddress)
}

func (c *AIClient) ListFreeModels(ctx context.Context) ([]string, error) {
	baseURL, providerType := c.configSnapshot()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	c.decorateRequest(req)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: models status %d", ErrAIUnavailable, resp.StatusCode)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var models []string
	for _, model := range out.Data {
		if providerType == ProviderOpenCode {
			if strings.Contains(model.ID, "free") {
				models = append(models, model.ID)
			}
			continue
		}
		if strings.TrimSpace(model.ID) != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (c *AIClient) Chat(ctx context.Context, model string, messages []ChatMessage) (ChatMessage, error) {
	requestMessages := buildTextOnlyRequestMessages(messages)
	reply, err := c.chatCompletion(ctx, model, requestMessages)
	if err != nil {
		return ChatMessage{}, err
	}
	return reply, nil
}

func (c *AIClient) ChatMultimodal(ctx context.Context, model string, messages []ChatRequestMessage) (ChatMessage, error) {
	resolved := make([]ChatRequestMessage, 0, len(messages))
	for _, message := range messages {
		if parts, ok := message.Content.([]ChatContentPart); ok {
			resolvedParts, err := c.resolveChatContentParts(ctx, parts)
			if err != nil {
				return ChatMessage{}, err
			}
			resolved = append(resolved, ChatRequestMessage{Role: message.Role, Content: resolvedParts})
			continue
		}
		resolved = append(resolved, message)
	}
	return c.chatCompletion(ctx, model, resolved)
}

func (c *AIClient) chatCompletion(ctx context.Context, model string, messages []ChatRequestMessage) (ChatMessage, error) {
	baseURL, _ := c.configSnapshot()
	model = strings.TrimSpace(model)
	if model == "" {
		return ChatMessage{}, ErrModelRequired
	}
	payload := chatCompletionRequest{
		Model:            model,
		Stream:           false,
		IncludeReasoning: false,
		Messages:         messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatMessage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.decorateRequest(req)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("%w: %v", ErrAIUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return ChatMessage{}, fmt.Errorf("%w: chat status %d", ErrAIUnavailable, resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatMessage{}, fmt.Errorf("%w: %v", ErrAIUnavailable, err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return ChatMessage{}, ErrAIUnavailable
	}
	reply := out.Choices[0].Message
	if reply.Role == "" {
		reply.Role = "assistant"
	}
	return reply, nil
}

func (c *AIClient) configSnapshot() (baseURL, providerType string) {
	if c == nil {
		return defaultAIBaseURL, ProviderOpenCode
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	baseURL = c.baseURL
	providerType = c.providerType
	if baseURL == "" {
		baseURL = defaultAIBaseURL
	}
	if providerType == "" {
		providerType = ProviderOpenCode
	}
	return baseURL, providerType
}

func (c *AIClient) decorateRequest(req *http.Request) {
	if c == nil || req == nil {
		return
	}
	c.mu.RLock()
	providerType := c.providerType
	apiKey := c.apiKey
	c.mu.RUnlock()
	if providerType == ProviderOpenAICompatible {
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		return
	}
	addOpenCodeHeaders(req)
}

func (c *AIClient) httpClient() *http.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.http
}

func (c *AIClient) resolveChatContentParts(ctx context.Context, parts []ChatContentPart) ([]ChatContentPart, error) {
	resolved := make([]ChatContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			resolved = append(resolved, part)
		case "image_url":
			if part.ImageURL == nil {
				return nil, errors.New("图片内容缺少URL")
			}
			if strings.HasPrefix(part.ImageURL.URL, "data:") {
				resolved = append(resolved, part)
				continue
			}
			dataURL, err := ResolveImageDataURL(ctx, c.httpClient(), part.ImageURL.URL)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, ChatContentPart{Type: "image_url", ImageURL: &ChatImageURL{URL: dataURL}})
		}
	}
	return resolved, nil
}

func newAIHTTPClient(proxyAddress string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyAddress = strings.TrimSpace(proxyAddress)
	if proxyAddress != "" {
		proxyURL, err := url.Parse("socks5://" + proxyAddress)
		if err == nil && proxyURL.Host != "" {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: transport}
}

func addOpenCodeHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer public")
	req.Header.Set("x-opencode-project", "proj_example")
	req.Header.Set("x-opencode-session", "sess_example")
	req.Header.Set("x-opencode-request", "msg_example")
	req.Header.Set("x-opencode-client", "cli")
}
