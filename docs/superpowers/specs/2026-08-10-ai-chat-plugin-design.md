# AI智能聊天插件设计

日期：2026-08-10

## 目标

将当前 Bee Go 插件模板改造成“AI智能聊天”插件：

- 插件名称：AI智能聊天
- 插件作者：周星星
- 插件版本：0.0.1
- 插件说明：一个简易且实用的AI智能聊天插件

插件通过本地网页完成主要配置，通过原生设置窗口手动控制本地 HTTP 服务，并在好友、群聊、频道消息中按固定前缀触发 AI 回复。

## 范围

本次实现包含：

- 本地轻量 HTTP 服务，基于 Go 标准库 `net/http`。
- 原生 Win32 设置窗口，白色底色，使用标准控件，不使用 GDI/GDI+ 绘制 UI。
- 网页端配置页和 JSON API。
- OpenAI 兼容模型列表与聊天接口客户端，默认参考易语言实现访问 `https://opencode.ai/zen/v1`。
- 本地配置与会话持久化。
- 好友、群聊、频道消息的前缀触发聊天。

不包含：

- HTTP 服务开机或插件启用后自动启动。
- 频道私信 Markdown 回复；当前 SDK 没有频道私信 Markdown 方法，本次只处理“频道消息”入口。
- 引入 HTTP 框架、中间件或前端构建工具。

## 配置

配置保存到插件数据目录下的 `config.json`，数据目录通过 `GetAppDataDir()` 获取。

配置字段：

- `port`：HTTP 服务端口，默认 `8765`。
- `model`：聊天模型名称，默认空；为空时聊天回复“请先设置对话大模型”。
- `system_prompt`：系统预设，默认使用简洁中文助手预设。
- `conversation_limit`：对话上限，默认 `42`，小于 `4` 时按 `4` 处理。
- `enable_friend`：是否处理好友消息，默认 `true`。
- `enable_group`：是否处理群聊消息，默认 `true`。
- `enable_channel`：是否处理频道消息，默认 `true`。

触发前缀固定为 `#`，不做网页配置。

## HTTP 服务

HTTP 服务默认不启动。用户点击设置窗口里的“启动HTTP服务”后启动，点击“停止HTTP服务”后停止。插件禁用或卸载时强制停止服务。

服务接口：

- `GET /`：配置页面。
- `GET /api/status`：返回服务状态、配置和访问地址。
- `POST /api/config`：保存网页配置。
- `GET /api/models`：拉取可用模型列表，只返回包含 `free` 的模型 ID。

端口检查在启动前执行。如果端口被占用，启动失败并在窗口状态中显示不可用原因。

## 设置窗口

设置窗口仍保持单实例、固定大小、居中、置顶和可关闭。

窗口使用标准 Win32 控件：

- 状态文本：显示“HTTP服务：运行中/未启动”。
- 端口输入框：输入 HTTP 端口。
- 端口检查文本：显示端口可用、不可用或当前服务使用中。
- 按钮：启动HTTP服务、停止HTTP服务、打开网址。

窗口背景使用系统白色窗口底色和标准控件，不再绘制卡片、色块、圆点或自定义文字层。

## 聊天行为

消息处理规则：

1. 只处理已启用工作区域的消息。
2. 消息必须以 `#` 开头。
3. 去掉前缀并 `strings.TrimSpace` 后得到用户输入。
4. 输入为空则不发送 AI 请求。
5. AI 回复使用自定义 Markdown 内容发送，即 `MarkdownMessage{Native: reply}`。

发送方式：

- 好友消息：`SendFriendMarkdown(friendID, MarkdownMessage{Native: reply}, false, false)`。
- 群聊消息：`SendGroupMarkdown(groupID, MarkdownMessage{Native: reply}, false)`。
- 频道消息：`SendChannelMarkdown(subChannelID, MarkdownMessage{Native: reply}, false)`。

插件处理完命中消息后返回 `MessageContinue`，避免改变其他插件投递行为。

## AI 客户端

接口参考易语言实现：

- 模型列表：`GET https://opencode.ai/zen/v1/models`
- 聊天补全：`POST https://opencode.ai/zen/v1/chat/completions`

请求头：

- `Authorization: Bearer public`
- `x-opencode-project: proj_example`
- `x-opencode-session: sess_example`
- `x-opencode-request: msg_example`
- `x-opencode-client: cli`

聊天请求体使用 OpenAI 兼容字段：

- `model`
- `stream: false`
- `include_reasoning: false`
- `messages`

失败处理：

- 模型为空：返回“请先设置对话大模型”。
- 网络失败、响应非 2xx、解析失败或回复为空：记录日志并返回“网络异常，请稍后重试……”。

## 会话与压缩

每个会话独立保存历史消息。会话键包含工作区域和来源 ID：

- 好友：`friend/<friendID>`
- 群聊：`group/<groupID>/<userID>`
- 频道：`channel/<subChannelID>/<userID>`

历史文件保存在插件数据目录的 `sessions/` 下，文件名使用安全编码，避免直接使用分隔符作为路径。

当历史消息数量达到 `conversation_limit` 时：

1. 使用现有历史加一条压缩提示发起总结请求。
2. 成功后将历史重置为：
   - `system`: 当前系统预设
   - `system`: 总结出的记忆概述
   - `user`: 当前用户输入
3. 总结失败时保留原历史并继续正常对话。

正常回复成功后，将 assistant 回复追加到历史并写回磁盘。

## 测试

新增单元测试覆盖：

- 配置默认值、保存和加载。
- 端口可用性检查。
- 模型列表过滤。
- 前缀触发和空输入处理。
- 聊天请求构造、模型为空错误、回复解析。
- 会话达到上限时的压缩行为。

验证命令：

- `go test ./...`
- `go vet ./...`

真实 Bee 环境仍需人工验证设置窗口、HTTP 服务控制、消息回调和 Markdown 发送。
