# 自定义 OpenAI 兼容接口支持 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让插件在保留 OPENCODE 默认行为的同时，支持配置自定义 OpenAI 兼容接口的 `base_url`、`api_key` 和 `model`，并且统一走固定的 `/chat/completions` 路径。

**Architecture:** 把接口差异收敛到 `AIClient` 的请求构造层和 `AIConfig` 的持久化配置层。默认仍使用 OPENCODE 的公共接口；当用户选择自定义模式时，客户端改为使用配置的 `base_url` 和 Bearer Key 发送同样的 OpenAI 兼容请求体，UI 只负责编辑这些字段，不承担协议逻辑。

**Tech Stack:** Go 1.22, `net/http`, 原生 Win32 设置窗口, 本地 HTML/JS 配置页, `go test ./...`

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

## Task 1: Extend config model for provider settings

**Files:**
- Modify: `config.go`
- Test: `config_test.go`

**Interfaces:**
- Consumes: `DefaultAIConfig`, `NormalizeAIConfig`, `LoadAIConfig`, `SaveAIConfig`
- Produces: `ProviderType`, `BaseURL`, `APIKey`, updated defaults and normalization rules

- [ ] **Step 1: Write the failing test**
  - Add a round-trip test that saves and loads a config with `provider_type: "openai_compatible"`, `base_url: "https://api.example.com/v1"`, and `api_key: "sk-test"`.
  - Add a normalization test that trims `base_url` and `api_key`, falls back to OPENCODE when `provider_type` is blank or invalid, and restores the default OPENCODE base URL when no custom URL is provided.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `go test ./... -run TestAIConfig`
  - Expected: fail because the new fields and normalization behavior do not exist yet.

- [ ] **Step 3: Write minimal implementation**
  - Add the new config fields and normalization logic, keeping existing JSON compatibility.
  - Preserve current defaults for the existing OPENCODE path.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `go test ./... -run TestAIConfig`
  - Expected: pass.

## Task 2: Make AIClient honor provider config

**Files:**
- Modify: `ai_client.go`
- Test: `ai_client_test.go`

**Interfaces:**
- Consumes: `AIConfig` fields `ProviderType`, `BaseURL`, `APIKey`
- Produces: `NewAIClientFromConfig(cfg AIConfig, httpClient *http.Client) *AIClient`, request headers for default and custom providers

- [ ] **Step 1: Write the failing test**
  - Add a test that a custom-configured client POSTs to `https://api.example.com/v1/chat/completions`.
  - Assert the request includes `Authorization: Bearer <api_key>` and still sends the same OpenAI-compatible JSON body.
  - Add a test that the default OPENCODE provider still sends the existing public headers.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `go test ./... -run TestAIClient`
  - Expected: fail because the client cannot switch auth/header behavior yet.

- [ ] **Step 3: Write minimal implementation**
  - Introduce provider-aware request decoration in `AIClient`.
  - Keep `/models` and `/chat/completions` fixed for compatibility, with model listing allowed to continue using the current endpoint when available.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `go test ./... -run TestAIClient`
  - Expected: pass.

## Task 3: Expose provider settings in the HTTP config page

**Files:**
- Modify: `http_server.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: updated `AIConfig`, `HTTPService.handleConfig`, `/api/status`
- Produces: form fields for provider type, base URL, and API key

- [ ] **Step 1: Write the failing test**
  - Add an HTTP handler test that POSTs a custom provider config and reads it back from `/api/status`.
  - Assert the API key is persisted but not echoed back in any public-facing HTML or JSON status payload.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `go test ./... -run TestHTTPService`
  - Expected: fail because the page and payload do not support the new fields yet.

- [ ] **Step 3: Write minimal implementation**
  - Add the new inputs to the HTML config page.
  - Update POST handling to save the new config fields and keep status responses safe for display.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `go test ./... -run TestHTTPService`
  - Expected: pass.

## Task 4: Update plugin docs and verify full test suite

**Files:**
- Modify: `README.md`
- Modify: `docs/AI开发指南.md` if the runtime behavior changes

**Interfaces:**
- Consumes: completed config/client/UI implementation
- Produces: updated user-facing setup guidance

- [ ] **Step 1: Run the full test suite**
  - Run: `go test ./...`
  - Expected: pass.

- [ ] **Step 2: Run static analysis**
  - Run: `go vet ./...`
  - Expected: pass.

- [ ] **Step 3: Update docs**
  - Document the new `provider_type`, `base_url`, and `api_key` fields and the fixed `/chat/completions` path.

