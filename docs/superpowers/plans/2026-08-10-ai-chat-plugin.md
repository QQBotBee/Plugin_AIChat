# AI智能聊天插件 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a lightweight Bee AI chat plugin with manual local HTTP configuration, prefix-triggered chat, persistent conversation memory, and custom Markdown replies.

**Architecture:** Keep Bee callback wiring in `plugin_main.go`, move AI/config/session/HTTP logic into focused Go files, and keep the settings window as a Windows-only native control host in `settings.go`. Use Go standard library only for HTTP serving, persistence, JSON, and outbound API calls.

**Tech Stack:** Go 1.22, `net/http`, `encoding/json`, Win32 APIs via `syscall`, existing Bee SDK Markdown methods.

## Global Constraints

- Plugin name: `AI智能聊天`.
- Plugin author: `周星星`.
- Plugin version: `0.0.1`.
- Plugin description: `一个简易且实用的AI智能聊天插件`.
- HTTP service must not auto-start; user starts it from the settings window.
- Disable/unload must force-stop HTTP service.
- Settings UI must use native controls, white background, and no GDI/GDI+ custom painting.
- HTTP implementation must use standard library `net/http`, no router framework or middleware.
- Trigger prefix is fixed to `#`.
- Replies must use custom Markdown content via `MarkdownMessage{Native: reply}`.
- Work areas: friend, group, channel messages only; no channel private Markdown reply.
- Config persists under `GetAppDataDir()` as `config.json`.
- Conversation files persist under `GetAppDataDir()/sessions/`.

---

## File Structure

- Create `ipc_message.go`: shared `IPCMessage` type so root package tests compile without the worker build tag.
- Modify `other/worker_runtime.go`: remove duplicate `IPCMessage` definition and reuse shared type.
- Create `config.go`: `AIConfig`, defaults, validation, load/save helpers, data directory injection.
- Create `ai_client.go`: OpenAI-compatible request/response structs, model listing, chat completion, error mapping.
- Create `sessions.go`: safe session filename generation, message history load/save, compression orchestration.
- Create `chat.go`: prefix parsing, work-area gating, session key creation, chat service orchestration, Markdown send helpers.
- Create `http_server.go`: local service manager, status/config/model endpoints, embedded HTML page.
- Modify `settings.go`: replace custom paint path with native controls and HTTP service controls.
- Modify `plugin_main.go`: metadata, lifecycle cleanup, message callback integration.
- Add tests next to code: `config_test.go`, `ai_client_test.go`, `sessions_test.go`, `chat_test.go`, `http_server_test.go`.

---

### Task 1: Restore Testable Baseline IPC Type

**Files:**
- Create: `ipc_message.go`
- Modify: `other/worker_runtime.go`
- Test: existing package build through `go test ./...`

**Interfaces:**
- Produces: `type IPCMessage struct { Type string; ID string; Event string; ArgsB64 []string; CommandB64 string; ValueB64 string; Result int; Error string }`

- [ ] **Step 1: Write the failing test**

No new test is needed; the current baseline command already fails because `bee_sdk.go` references `IPCMessage` outside the `bee_worker_runtime` build tag.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL with `undefined: IPCMessage`.

- [ ] **Step 3: Write minimal implementation**

Create `ipc_message.go` in package `main`:

```go
package main

type IPCMessage struct {
	Type       string   `json:"type"`
	ID         string   `json:"id,omitempty"`
	Event      string   `json:"event,omitempty"`
	ArgsB64    []string `json:"args_b64,omitempty"`
	CommandB64 string   `json:"command_b64,omitempty"`
	ValueB64   string   `json:"value_b64,omitempty"`
	Result     int      `json:"result,omitempty"`
	Error      string   `json:"error,omitempty"`
}
```

Remove the same type block from `other/worker_runtime.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS for package build and any existing tests.

- [ ] **Step 5: Commit**

```bash
git add ipc_message.go other/worker_runtime.go
git commit -m "fix: share ipc message type"
```

---

### Task 2: Config Persistence

**Files:**
- Create: `config.go`
- Test: `config_test.go`

**Interfaces:**
- Produces: `type AIConfig`, `func DefaultAIConfig() AIConfig`, `func NormalizeAIConfig(AIConfig) AIConfig`, `func LoadAIConfig(path string) (AIConfig, error)`, `func SaveAIConfig(path string, cfg AIConfig) error`

- [ ] **Step 1: Write failing tests**

Tests:

```go
func TestDefaultAIConfigHasExpectedRuntimeDefaults(t *testing.T)
func TestLoadAIConfigMissingFileReturnsDefaults(t *testing.T)
func TestSaveAndLoadAIConfigRoundTrip(t *testing.T)
func TestNormalizeAIConfigClampsInvalidPortAndLimit(t *testing.T)
```

Expected checks: port `8765`, limit `42`, friend/group/channel enabled, model empty, invalid port becomes `8765`, limit below `4` becomes `4`.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`
Expected: FAIL because `AIConfig` and helpers are undefined.

- [ ] **Step 3: Implement config helpers**

Use JSON with indented output. Missing file returns defaults. Save creates parent directories with `0755` and writes file with `0644`.

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: add ai config persistence"
```

---

### Task 3: AI Client

**Files:**
- Create: `ai_client.go`
- Test: `ai_client_test.go`

**Interfaces:**
- Consumes: `ChatMessage` from this task.
- Produces: `type ChatMessage struct { Role string; Content string }`, `type AIClient`, `func NewAIClient(baseURL string, httpClient *http.Client) *AIClient`, `func (c *AIClient) ListFreeModels(ctx context.Context) ([]string, error)`, `func (c *AIClient) Chat(ctx context.Context, model string, messages []ChatMessage) (ChatMessage, error)`

- [ ] **Step 1: Write failing tests**

Tests:

```go
func TestAIClientListFreeModelsFiltersFreeIDs(t *testing.T)
func TestAIClientChatPostsOpenAICompatiblePayload(t *testing.T)
func TestAIClientChatRejectsEmptyModel(t *testing.T)
func TestAIClientChatReturnsNetworkFallbackOnEmptyChoice(t *testing.T)
```

Use `httptest.Server`. Assert request headers include the opencode public headers and chat payload has `stream:false`, `include_reasoning:false`, and the requested model/messages.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`
Expected: FAIL because AI client symbols are undefined.

- [ ] **Step 3: Implement client**

Set default base URL to `https://opencode.ai/zen/v1`. For chat, return assistant `ChatMessage{Role:"assistant", Content: content}`. Use exported sentinel errors `ErrModelRequired` and `ErrAIUnavailable`.

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ai_client.go ai_client_test.go
git commit -m "feat: add opencode ai client"
```

---

### Task 4: Sessions and Compression

**Files:**
- Create: `sessions.go`
- Test: `sessions_test.go`

**Interfaces:**
- Consumes: `ChatMessage`, `AIClient.Chat`.
- Produces: `type SessionStore`, `func NewSessionStore(root string) *SessionStore`, `func (s *SessionStore) Load(key string) ([]ChatMessage, error)`, `func (s *SessionStore) Save(key string, messages []ChatMessage) error`, `func SessionKey(kind, sourceID, userID string) string`, `func BuildMessagesForTurn(ctx context.Context, client chatCompleter, cfg AIConfig, history []ChatMessage, userInput string) ([]ChatMessage, error)`

- [ ] **Step 1: Write failing tests**

Tests:

```go
func TestSessionStoreRoundTripUsesSafeFilename(t *testing.T)
func TestSessionKeyIncludesKindSourceAndUser(t *testing.T)
func TestBuildMessagesForTurnStartsWithSystemPrompt(t *testing.T)
func TestBuildMessagesForTurnCompressesWhenLimitReached(t *testing.T)
```

Use a fake completer for compression that returns `ChatMessage{Role:"assistant", Content:"[记忆概述：用户喜欢Go]"}`.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`
Expected: FAIL because session symbols are undefined.

- [ ] **Step 3: Implement sessions**

Use URL query escaping or base64 URL encoding for filenames. Compression prompt text must match the design intent: ask for concise memory summary and strict `[记忆概述：内容]` format.

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add sessions.go sessions_test.go
git commit -m "feat: add ai chat sessions"
```

---

### Task 5: Chat Orchestration and Callback Integration

**Files:**
- Create: `chat.go`
- Modify: `plugin_main.go`
- Test: `chat_test.go`

**Interfaces:**
- Consumes: `AIConfig`, `SessionStore`, `AIClient`, `MarkdownMessage`.
- Produces: `func ParseAIInput(message string) (string, bool)`, `func HandleAIChat(ctx context.Context, service *ChatService, target ChatTarget, message string, send func(string) error) bool`

- [ ] **Step 1: Write failing tests**

Tests:

```go
func TestParseAIInputRequiresHashPrefix(t *testing.T)
func TestParseAIInputTrimsAfterPrefix(t *testing.T)
func TestChatServiceSkipsDisabledWorkArea(t *testing.T)
func TestChatServiceSendsMarkdownNativeReply(t *testing.T)
```

The Markdown test should assert the sent payload contains the AI reply string passed as `MarkdownMessage.Native`.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`
Expected: FAIL because chat orchestration symbols are undefined.

- [ ] **Step 3: Implement chat orchestration**

Load config at message time, check work-area enablement, parse prefix, build session key, load history, call AI, save updated history, send Markdown reply. Return `true` only when a message was handled.

- [ ] **Step 4: Modify callbacks**

In `onPrivateMessage`, `onGroupMessage`, and `onChannelMessage`, create `BeeAPI`, call `HandleAIChat`, and use:

```go
bee.ctx.SendFriendMarkdown(friendID, MarkdownMessage{Native: reply}, false, false)
bee.ctx.SendGroupMarkdown(groupID, MarkdownMessage{Native: reply}, false)
bee.ctx.SendChannelMarkdown(subChannelID, MarkdownMessage{Native: reply}, false)
```

Keep return value `MessageContinue`.

- [ ] **Step 5: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add chat.go chat_test.go plugin_main.go
git commit -m "feat: handle prefixed ai chat"
```

---

### Task 6: HTTP Service and Web Config

**Files:**
- Create: `http_server.go`
- Test: `http_server_test.go`

**Interfaces:**
- Consumes: `AIConfig`, `LoadAIConfig`, `SaveAIConfig`, `AIClient.ListFreeModels`.
- Produces: `type HTTPService`, `func NewHTTPService(configPath string, client *AIClient) *HTTPService`, `func (s *HTTPService) Start(port int) error`, `func (s *HTTPService) Stop(ctx context.Context) error`, `func (s *HTTPService) Status() HTTPServiceStatus`, `func IsPortAvailable(port int) bool`, `func ConfigURL(port int) string`

- [ ] **Step 1: Write failing tests**

Tests:

```go
func TestIsPortAvailableDetectsBoundPort(t *testing.T)
func TestHTTPServiceStatusReportsStoppedByDefault(t *testing.T)
func TestHTTPServiceConfigRoundTrip(t *testing.T)
func TestHTTPServiceModelsEndpointReturnsFreeModels(t *testing.T)
```

Use `httptest` against the handler for endpoint tests and real local listener only for port availability.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`
Expected: FAIL because HTTP service symbols are undefined.

- [ ] **Step 3: Implement service**

Use `http.ServeMux`, `net.Listen("tcp", "127.0.0.1:<port>")`, `http.Server`, mutex-protected status, and no framework. HTML page can be embedded as a string with form controls and small vanilla JavaScript.

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add http_server.go http_server_test.go
git commit -m "feat: add local config http service"
```

---

### Task 7: Native Settings Window Controls

**Files:**
- Modify: `settings.go`
- Test: existing compile through `go test ./...`

**Interfaces:**
- Consumes: package-level HTTP service controls from Task 6.
- Produces: `showSettingsWindow()` window with native controls and `closeSettingsWindow()` cleanup unchanged for callers.

- [ ] **Step 1: Write failing compile-oriented guard**

No behavioral unit test is practical for Win32 UI in this repo. Existing compile verification must fail if symbols are missing after replacing custom painting.

- [ ] **Step 2: Run tests before edit**

Run: `go test ./...`
Expected: PASS from previous task.

- [ ] **Step 3: Replace custom painting with controls**

Remove GDI+ startup and paint helpers. Add Win32 procedures for `CreateWindowExW` controls, `SetWindowTextW`, `GetWindowTextW`, `EnableWindow`, `ShellExecuteW`, and `WM_COMMAND` handling. Use white `COLOR_WINDOW` background. Buttons: start, stop, open URL. Edit: port. Labels: status and port availability.

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add settings.go
git commit -m "feat: replace settings ui with native controls"
```

---

### Task 8: Lifecycle Wiring, Vet, and Final Verification

**Files:**
- Modify: `plugin_main.go`
- Maybe Modify: `docs/设置窗口开发规范.md`

**Interfaces:**
- Consumes: `StopPluginHTTPService`, `InitializePluginServices`.
- Produces: lifecycle that initializes paths and force-stops HTTP service on disable/unload.

- [ ] **Step 1: Wire lifecycle**

Update plugin metadata. In `onInitialize`, resolve app data dir and initialize config/session/http service globals. In `onDisable` and `onUnload`, stop HTTP service before closing/finishing.

- [ ] **Step 2: Update docs if needed**

If `docs/设置窗口开发规范.md` still says GDI+ draws the UI, revise it to describe native controls.

- [ ] **Step 3: Run gofmt**

Run: `gofmt -w *.go other/worker_runtime.go`

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Run vet**

Run: `go vet ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add plugin_main.go docs/设置窗口开发规范.md
git commit -m "chore: finalize ai chat plugin lifecycle"
```

