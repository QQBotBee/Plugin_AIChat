# 图片输入多模态支持 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持从消息文本中解析 `[pic=...,url=...]` 图片标记，下载图片到内存并转成 base64，按 OpenAI 兼容多模态格式发送给模型。

**Architecture:** 保留现有好友、群聊、频道入口和会话存储；新增消息解析器负责拆分文本与图片片段，新增图片临时加载器负责在内存中下载并做 10MB 上限校验，新增多模态请求构造用于把文本和图片 part 送进 `/chat/completions`。图片不落盘，不改现有纯文本聊天路径。

**Tech Stack:** Go 1.22, `net/http`, `encoding/base64`, 本地单元测试, OpenAI-compatible chat payloads

## Global Constraints

- Windows 10/11
- Go 1.22 或更高版本
- Zig 已加入 `PATH`
- Windows CMD
- 构建目标固定为 Windows/386
- `build.bat` 必须保持 GBK/CP936 编码和 CRLF 换行
- `other/BeePlugin.def` 必须保持 GBK 编码
- 不要提交 `build/`、`temp/`、DLL、EXE、RES、LIB、PDB 等构建产物
- 持久化数据应写入 `GetAppDataDir()` 返回的插件数据目录
- 单张图片原始字节上限为 10MB
- 图片只允许内存临时处理，不写入本地文件

## Task 1: Parse image markers from incoming text

**Files:**
- Create: `message_media.go`
- Test: `message_media_test.go`

**Interfaces:**
- Produces: `type MessageContentPart`, `func ParseMessageContent(message string) ParsedMessageContent`

- [ ] **Step 1: Write the failing test**
  - Add a test that parses `哈哈哈[pic=1,url=https://a][pic=2,url=https://b]尾巴`.
  - Assert the returned text history becomes `哈哈哈[图片][图片]尾巴`.
  - Assert the returned parts keep order as text, image, image, text.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `go test ./... -run TestParseMessageContent`
  - Expected: fail because the parser does not exist yet.

- [ ] **Step 3: Write minimal implementation**
  - Add a parser that recognizes `[pic=...,url=...]` tokens and splits surrounding text without altering order.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `go test ./... -run TestParseMessageContent`
  - Expected: pass.

## Task 2: Add in-memory image download and data URL conversion

**Files:**
- Modify: `ai_client.go`
- Modify: `message_media.go`
- Test: `message_media_test.go`

**Interfaces:**
- Consumes: `AIClient.httpClient()`
- Produces: `func ResolveImageDataURL(ctx context.Context, client *http.Client, rawURL string) (string, error)`

- [ ] **Step 1: Write the failing test**
  - Add a test that serves a small PNG from `httptest.Server` and expects a `data:image/png;base64,...` URL back.
  - Add a test that serves a body larger than 10MB and expects a size error.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `go test ./... -run TestResolveImageDataURL`
  - Expected: fail because the helper does not exist yet.

- [ ] **Step 3: Write minimal implementation**
  - Download in memory only, inspect `Content-Type`, reject bodies above 10MB, base64 encode the payload, and return a data URL.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `go test ./... -run TestResolveImageDataURL`
  - Expected: pass.

## Task 3: Send multimodal chat payloads

**Files:**
- Modify: `ai_client.go`
- Modify: `chat.go`
- Test: `ai_client_test.go`
- Test: `chat_test.go`

**Interfaces:**
- Produces: `type ChatContentPart`, `type ChatRequestMessage`, `func (c *AIClient) ChatMultimodal(ctx context.Context, model string, messages []ChatRequestMessage) (ChatMessage, error)`

- [ ] **Step 1: Write the failing test**
  - Add a request-capture test that checks a multimodal request contains a `content` array with both `text` and `image_url` parts.
  - Add an integration-style chat test that sends a message containing one image token and verifies the AI request body includes a base64 data URL.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `go test ./... -run TestAIClient|TestChatService`
  - Expected: fail because multimodal payloads are not wired yet.

- [ ] **Step 3: Write minimal implementation**
  - Teach `ChatService.Handle` to parse image markers, resolve images in memory, and call `ChatMultimodal` when images exist.
  - Keep the plain text path unchanged when no images are present.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `go test ./... -run TestAIClient|TestChatService`
  - Expected: pass.

## Task 4: Update docs and run full verification

**Files:**
- Modify: `README.md`
- Modify: `docs/AI开发指南.md`

**Interfaces:**
- Consumes: completed multimodal chat support

- [ ] **Step 1: Run the full test suite**
  - Run: `go test ./...`
  - Expected: pass.

- [ ] **Step 2: Run static analysis**
  - Run: `go vet ./...`
  - Expected: pass.

- [ ] **Step 3: Update docs**
  - Document `[pic=...,url=...]` support, the 10MB cap, and that images are handled only in memory.

