package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

type AIClient struct {
	baseURL string
	http    *http.Client
}

func NewAIClient(baseURL string, httpClient *http.Client) *AIClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultAIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &AIClient{baseURL: baseURL, http: httpClient}
}

func (c *AIClient) ListFreeModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	addOpenCodeHeaders(req)
	resp, err := c.http.Do(req)
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
		if strings.Contains(model.ID, "free") {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (c *AIClient) Chat(ctx context.Context, model string, messages []ChatMessage) (ChatMessage, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return ChatMessage{}, ErrModelRequired
	}
	payload := struct {
		Model            string        `json:"model"`
		Stream           bool          `json:"stream"`
		IncludeReasoning bool          `json:"include_reasoning"`
		Messages         []ChatMessage `json:"messages"`
	}{
		Model:            model,
		Stream:           false,
		IncludeReasoning: false,
		Messages:         messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatMessage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	addOpenCodeHeaders(req)
	resp, err := c.http.Do(req)
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

func addOpenCodeHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer public")
	req.Header.Set("x-opencode-project", "proj_example")
	req.Header.Set("x-opencode-session", "sess_example")
	req.Header.Set("x-opencode-request", "msg_example")
	req.Header.Set("x-opencode-client", "cli")
}
