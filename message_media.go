package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const maxImageBytes = 10 << 20

var errImageTooLarge = errors.New("图片超过10MB")
var imageMarkerRegexp = regexp.MustCompile(`\[pic=([^,\]]+),url=([^\]]+)\]`)

type MessageContentPart struct {
	Type string
	Text string
	URL  string
}

type ParsedMessageContent struct {
	Text  string
	Parts []MessageContentPart
}

type ChatImageURL struct {
	URL string `json:"url"`
}

type ChatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ChatImageURL `json:"image_url,omitempty"`
}

type ChatRequestMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func ParseMessageContent(message string) ParsedMessageContent {
	var parsed ParsedMessageContent
	if message == "" {
		return parsed
	}
	indexes := imageMarkerRegexp.FindAllStringSubmatchIndex(message, -1)
	if len(indexes) == 0 {
		return ParsedMessageContent{
			Text:  strings.TrimSpace(message),
			Parts: []MessageContentPart{{Type: "text", Text: message}},
		}
	}
	cursor := 0
	for _, loc := range indexes {
		if len(loc) < 6 {
			continue
		}
		if loc[0] > cursor {
			segment := message[cursor:loc[0]]
			if segment != "" {
				parsed.Parts = append(parsed.Parts, MessageContentPart{Type: "text", Text: segment})
				parsed.Text += segment
			}
		}
		parsed.Parts = append(parsed.Parts, MessageContentPart{Type: "image", URL: message[loc[4]:loc[5]]})
		parsed.Text += "[图片]"
		cursor = loc[1]
	}
	if cursor < len(message) {
		segment := message[cursor:]
		if segment != "" {
			parsed.Parts = append(parsed.Parts, MessageContentPart{Type: "text", Text: segment})
			parsed.Text += segment
		}
	}
	parsed.Text = strings.TrimSpace(parsed.Text)
	if parsed.Text == "" && len(parsed.Parts) > 0 {
		imageCount := 0
		for _, part := range parsed.Parts {
			if part.Type == "image" {
				imageCount++
			}
		}
		if imageCount > 0 {
			parsed.Text = strings.Repeat("[图片]", imageCount)
		}
	}
	return parsed
}

func BuildChatContentParts(ctx context.Context, client *http.Client, parts []MessageContentPart) ([]ChatContentPart, error) {
	out := make([]ChatContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			out = append(out, ChatContentPart{Type: "text", Text: part.Text})
		case "image":
			dataURL, err := ResolveImageDataURL(ctx, client, part.URL)
			if err != nil {
				return nil, err
			}
			out = append(out, ChatContentPart{Type: "image_url", ImageURL: &ChatImageURL{URL: dataURL}})
		}
	}
	return out, nil
}

func ResolveImageDataURL(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("图片地址无效")
	}
	if client == nil {
		client = &http.Client{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("图片下载失败: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxImageBytes {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", errImageTooLarge
	}
	limited := io.LimitReader(resp.Body, maxImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(data) > maxImageBytes {
		return "", errImageTooLarge
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func buildTextOnlyRequestMessages(messages []ChatMessage) []ChatRequestMessage {
	out := make([]ChatRequestMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, ChatRequestMessage{Role: message.Role, Content: message.Content})
	}
	return out
}

func replaceLastUserContent(messages []ChatRequestMessage, content []ChatContentPart) []ChatRequestMessage {
	if len(messages) == 0 {
		return messages
	}
	out := append([]ChatRequestMessage(nil), messages...)
	out[len(out)-1].Content = content
	return out
}

func isImageLimitError(err error) bool {
	return errors.Is(err, errImageTooLarge)
}
