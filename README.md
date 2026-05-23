# chat2responses

> Chat Completions → Responses API Proxy

[English](#english) · [中文](#chinese)

---

<a id="english"></a>

# chat2responses

> Chat Completions → Responses API Proxy

A transparent protocol translation proxy that bridges OpenAI's Chat Completions API and Responses API, enabling modern Responses API clients (like Codex CLI) to work with any Chat Completions-compatible backend.

---

## Background

OpenAI Codex CLI uses the Responses API (`/v1/responses`) as its standard interface. However, most model providers—including self-hosted models, open-source LLMs, and many commercial APIs—only support the Chat Completions API (`/v1/chat/completions`). **chat2responses** fills this gap: it sits transparently between the client and upstream, translating protocol in real-time.

---

## Protocol Comparison

| Dimension | Chat Completions API | Responses API |
|-----------|---------------------|---------------|
| Endpoint | `/v1/chat/completions` | `/v1/responses` |
| Message structure | `messages[]` array | `input` + `instructions` |
| System role | `system` | `instructions` field |
| Tool calls | `tool_calls` (flat) | `function_call` + `function_call_output` |
| Output format | `choices[].message` | `output[]` array |
| Streaming | SSE `data: {"choices":[...]}` | SSE `data: {"type": "response.output_text.delta"}` |

---

## How It Works

The proxy sits between your Responses API client and any Chat Completions upstream:

```
                    Responses API              Chat Completions
Client (Codex CLI) \u2501\u2501\u2501\u2501\u2501\u252b\u2501\u2501\u2501\u2501\u2501\u2592 chat2responses \u2501\u2501\u2501\u2501\u2501\u252b\u2501\u2501\u2501\u2501\u2501\u2592 Upstream (OpenAI, local LLM, ...)
```

- **Request**: Converts `instructions` → `system` message, `input` items → `messages` array, tools → Chat Completions tool format
- **Response** (non-streaming): Maps `choices[].message` → `output[]` items, remaps token usage
- **Streaming**: Translates Chat SSE chunks → Responses SSE events (`output_text.delta`, `function_call_arguments.delta`, etc.)

---

## Features

- **Protocol translation** — Bidirectional mapping between Responses API and Chat Completions API
- **Session isolation** — Each `previous_response_id` chain is tracked independently, allowing multiple clients (e.g. Codex CLI + Codex Desktop) to maintain separate conversations simultaneously
- **Streaming** — Native SSE support with real-time token, reasoning, and function call argument streaming
- **Function calling** — Full round-trip conversion of Responses `function_call` / `function_call_output` to assistant `tool_calls` / tool messages
- **Tool definitions** — Passthrough tool definitions with parallel tool call support
- **Setup wizard** — Interactive CLI wizard for quick configuration
- **Codex CLI integration** — Auto-configures Codex CLI to use the proxy
- **Multi-platform** — Prebuilt binaries for Linux, macOS (Intel + Apple Silicon), and Windows
- **Docker** — Minimal scratch-based image

---

## Quick Start

### Download

Prebuilt binaries are available on the [Releases page](https://github.com/fooyii/chat2responses/releases):

| Platform | Binary |
|----------|--------|
| Linux x86_64 | `chat2responses-linux-amd64` |
| macOS Intel | `chat2responses-darwin-amd64` |
| macOS Apple Silicon | `chat2responses-darwin-arm64` |
| Windows x86_64 | `chat2responses-windows-amd64.exe` |

### Run

```bash
# Start with setup wizard (creates config.json interactively)
./chat2responses

# Or start directly with existing config
./chat2responses serve
```

The proxy will start on `http://0.0.0.0:8000` by default.

---

## Configuration

### Config file

```json
{
  "upstream": {
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-..."
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8000
  },
  "model": {
    "default_model": "gpt-4o"
  },
  "debug": false
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `upstream.base_url` | string | — | Upstream Chat Completions API base URL |
| `upstream.api_key` | string | — | API key |
| `server.host` | string | `"0.0.0.0"` | Listen address |
| `server.port` | int | `8000` | Listen port |
| `model.default_model` | string | — | Default model name |
| `debug` | bool | `false` | Debug mode |

### Config load order

1. `./config.json`
2. `$XDG_CONFIG_HOME/chat2responses/config.json` (falls back to `$HOME/.config/`)
3. `/etc/chat2responses/config.json`

---

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/responses` | POST | Responses API proxy entrypoint |
| `/v1/models` | GET | Upstream model list passthrough |
| `/health` | GET | Health check |

### Protocol Mapping

**Request (Responses API → Chat Completions):**

| Responses API | Chat Completions |
|--------------|-----------------|
| `instructions` | System message |
| `input` (string) | User message |
| `input` (array) | Multiple messages |
| `input[n].role: developer` | `role: system` |
| `function_call` input item | Assistant `tool_calls` |
| `function_call_output` input item | Tool message |
| `max_output_tokens` | `max_tokens` |
| `tools[].name` (with empty function) | `tools[].function.name` |

**Response (Chat Completions → Responses API):**

| Chat Completions | Responses API |
|-----------------|---------------|
| `choices[].message.content` | `output[].content[0].text` |
| `choices[].message.tool_calls` | `output[].function_call` |
| `usage.prompt_tokens` | `usage.input_tokens` |
| `usage.completion_tokens` | `usage.output_tokens` |

---

## Codex CLI Integration

Use the setup wizard to automatically configure Codex CLI, or configure manually:

```bash
export CODEX_API_KEY="your-api-key"
export CODEX_BASE_URL="http://127.0.0.1:8000/v1"
codex
```

To restore the original Codex CLI configuration:

```bash
./chat2responses restore
```

---

## Build from Source

### Prerequisites

- Go 1.21+
- make

### Build

```bash
git clone https://github.com/fooyii/chat2responses.git
cd chat2responses
make build
```

Cross-compile all platforms:

```bash
make build-all
```

Artifacts are placed in `release/`.

### Docker

```bash
docker build -t chat2responses .
docker run -p 8000:8000 -v $(pwd)/config.json:/config.json chat2responses
```

---

## License

MIT

---

<a id="chinese"></a>

# chat2responses

> Chat Completions → Responses API 协议转换代理

一个透明的协议转换代理，桥接 OpenAI Chat Completions API 和 Responses API，让现代 Responses API 客户端（如 Codex CLI）能够接入任何 Chat Completions 兼容的后端。

---

## 背景

OpenAI Codex CLI 使用 Responses API（`/v1/responses`）作为其标准接口。然而大多数模型提供商——包括自部署模型、开源 LLM 和许多商业 API——仅支持 Chat Completions API（`/v1/chat/completions`）。**chat2responses** 填补了这个断层：它透明地居于客户端和上游之间，实时转换协议。

---

## 协议对比

| 维度 | Chat Completions API | Responses API |
|-----------|---------------------|---------------|
| 端点 | `/v1/chat/completions` | `/v1/responses` |
| 消息结构 | `messages[]` 数组 | `input` + `instructions` |
| 系统角色 | `system` | `instructions` 字段 |
| 工具调用 | `tool_calls`（平面结构） | `function_call` + `function_call_output` |
| 输出格式 | `choices[].message` | `output[]` 数组 |
| 流式 | SSE `data: {"choices":[...]}` | SSE `data: {"type": "response.output_text.delta"}` |

---

## 工作原理

代理处于 Responses API 客户端和 Chat Completions 上游之间：

```
                    Responses API              Chat Completions
客户端 (Codex CLI) \u2501\u2501\u2501\u2501\u2501\u252b\u2501\u2501\u2501\u2501\u2501\u2592 chat2responses \u2501\u2501\u2501\u2501\u2501\u252b\u2501\u2501\u2501\u2501\u2501\u2592 上游 (OpenAI、本地 LLM, ...)
```

- **请求**：`instructions` → `system` 消息，`input` 项 → `messages` 数组，工具定义转换为 Chat Completions 格式
- **响应**（非流式）：`choices[].message` → `output[]` 项，重新映射 token 用量
- **流式**：Chat SSE 块 → Responses SSE 事件（`output_text.delta`、`function_call_arguments.delta` 等）

---

## 功能特性

- **协议转换** — Responses API 与 Chat Completions API 双向映射
- **会话隔离** — 每个 `previous_response_id` 链独立追踪，多个客户端（如 Codex CLI + Codex Desktop）可同时维持独立对话
- **流式支持** — 原生 SSE 流式实时转发 token、推理内容和函数调用参数
- **函数调用** — Responses `function_call` / `function_call_output` 与 assistant `tool_calls` / 工具消息全双向转换
- **工具定义** — 透传工具定义，支持并行工具调用
- **设置向导** — 交互式命令行向导，快速配置
- **Codex CLI 集成** — 自动配置 Codex CLI 使用代理
- **多平台** — 预编译 Linux、macOS（Intel + Apple Silicon）和 Windows 二进制
- **Docker** — 基于 scratch 的最小化镜像

---

## 快速开始

### 下载

预编译二进制文件可在 [Releases 页面](https://github.com/fooyii/chat2responses/releases) 获取：

| 平台 | 文件 |
|----------|--------|
| Linux x86_64 | `chat2responses-linux-amd64` |
| macOS Intel | `chat2responses-darwin-amd64` |
| macOS Apple Silicon | `chat2responses-darwin-arm64` |
| Windows x86_64 | `chat2responses-windows-amd64.exe` |

### 运行

```bash
# 启动设置向导（交互式创建 config.json）
./chat2responses

# 或直接启动（需 config.json 已存在）
./chat2responses serve
```

默认启动在 `http://0.0.0.0:8000`。

---

## 配置

### 配置文件

```json
{
  "upstream": {
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-..."
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8000
  },
  "model": {
    "default_model": "gpt-4o"
  },
  "debug": false
}
```

| 字段 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| `upstream.base_url` | string | — | 上游 Chat Completions API 地址 |
| `upstream.api_key` | string | — | API 密钥 |
| `server.host` | string | `"0.0.0.0"` | 监听地址 |
| `server.port` | int | `8000` | 监听端口 |
| `model.default_model` | string | — | 默认模型名称 |
| `debug` | bool | `false` | 调试模式 |

### 配置加载顺序

1. `./config.json`
2. `$XDG_CONFIG_HOME/chat2responses/config.json`（未设置时回退到 `$HOME/.config/`）
3. `/etc/chat2responses/config.json`

---

## API 端点

| 端点 | 方法 | 说明 |
|----------|--------|------|
| `/v1/responses` | POST | Responses API 代理入口 |
| `/v1/models` | GET | 上游模型列表透传 |
| `/health` | GET | 健康检查 |

### 协议映射

**请求（Responses API → Chat Completions）：**

| Responses API | Chat Completions |
|--------------|-----------------|
| `instructions` | System 消息 |
| `input`（字符串）| User 消息 |
| `input`（数组）| 多条消息 |
| `input[n].role: developer` | `role: system` |
| `function_call` 输入项 | Assistant `tool_calls` |
| `function_call_output` 输入项 | Tool 消息 |
| `max_output_tokens` | `max_tokens` |
| `tools[].name`（无 function 对象） | `tools[].function.name` |

**响应（Chat Completions → Responses API）：**

| Chat Completions | Responses API |
|-----------------|---------------|
| `choices[].message.content` | `output[].content[0].text` |
| `choices[].message.tool_calls` | `output[].function_call` |
| `usage.prompt_tokens` | `usage.input_tokens` |
| `usage.completion_tokens` | `usage.output_tokens` |

---

## Codex CLI 集成

使用设置向导自动配置 Codex CLI，或手动配置：

```bash
export CODEX_API_KEY="your-api-key"
export CODEX_BASE_URL="http://127.0.0.1:8000/v1"
codex
```

恢复原始 Codex CLI 配置：

```bash
./chat2responses restore
```

---

## 自行编译

### 前置要求

- Go 1.21+
- make

### 编译

```bash
git clone https://github.com/fooyii/chat2responses.git
cd chat2responses
make build
```

交叉编译所有平台：

```bash
make build-all
```

产物位于 `release/` 目录。

### Docker

```bash
docker build -t chat2responses .
docker run -p 8000:8000 -v $(pwd)/config.json:/config.json chat2responses
```

---

## 协议

MIT
