# chat2responses

> **Chat Completions → Responses API Proxy**

[English](#english) | [中文](#chinese)

---

<a id="english"></a>

# chat2responses

> **Chat Completions → Responses API Proxy**

A lightweight, zero-dependency proxy server that translates OpenAI's **Chat Completions API** requests into the **Responses API** format. Works with any OpenAI-compatible upstream, including OpenAI, Azure OpenAI, and local LLM servers.

---

## Overview

`chat2responses` sits between your application and any OpenAI-compatible Chat Completions endpoint. It exposes a **Responses API** endpoint (`POST /v1/responses`) and translates requests on the fly — converting input items, tools, streaming events, and function calls between the two API formats.

Built primarily for use with [Codex CLI](https://github.com/openai/codex), which uses the Responses API natively. Point Codex at `chat2responses`, and it can talk to any Chat Completions upstream without modification.

## Features

- **Protocol translation**: Responses API ↔ Chat Completions API — bidirectional mapping for requests and responses
- **Session isolation**: Each `previous_response_id` chain is tracked independently, allowing multiple clients (e.g. Codex CLI + Codex Desktop) to maintain separate conversations simultaneously
- **Streaming support**: Real-time SSE conversion (text deltas, reasoning content, function call arguments)
- **Tool/function call handling**: Correctly maps Responses API tools and tool_choice to Chat Completions format, and converts tool calls back
- **Multi-modal input**: Converts `input_text` and `input_image` content blocks into chat messages
- **Codex CLI integration**: Interactive setup wizard that configures Codex to use the proxy automatically
- **Multi-platform binaries**: Prebuilt for Linux, macOS (Intel + Apple Silicon), and Windows
- **Docker support**: Minimal scratch-based image

## Installation

### Download a release

Grab the latest binary for your platform from the [releases page](https://github.com/fooyii/chat2responses/releases).

```bash
# macOS (Apple Silicon)
curl -LO https://github.com/fooyii/chat2responses/releases/latest/download/chat2responses-darwin-arm64
chmod +x chat2responses-darwin-arm64
sudo mv chat2responses-darwin-arm64 /usr/local/bin/chat2responses

# macOS (Intel)
curl -LO https://github.com/fooyii/chat2responses/releases/latest/download/chat2responses-darwin-amd64
chmod +x chat2responses-darwin-amd64
sudo mv chat2responses-darwin-amd64 /usr/local/bin/chat2responses

# Linux
curl -LO https://github.com/fooyii/chat2responses/releases/latest/download/chat2responses-linux-amd64
chmod +x chat2responses-linux-amd64
sudo mv chat2responses-linux-amd64 /usr/local/bin/chat2responses
```

### Build from source

Requires Go 1.21+.

```bash
git clone https://github.com/fooyii/chat2responses.git
cd chat2responses
make build
```

### Docker

```bash
docker build -t chat2responses .
docker run -p 8000:8000 -v $(pwd)/config.json:/config.json chat2responses
```

## Quick Start

### 1. Configure

Create a `config.json`:

```json
{
  "upstream": {
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-your-api-key-here"
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8000
  },
  "model": {
    "default_model": "gpt-4o"
  }
}
```

Or run the interactive setup wizard:

```bash
./chat2responses setup
```

### 2. Start the server

```bash
./chat2responses serve
```

The proxy starts on `http://0.0.0.0:8000`.

### 3. Use it

Send a Responses API request:

```bash
curl http://localhost:8000/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "input": "What is the capital of France?",
    "instructions": "Answer concisely."
  }'
```

Streaming:

```bash
curl http://localhost:8000/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "input": "Count from 1 to 5.",
    "stream": true
  }'
```

### 4. Use with Codex CLI

The setup wizard can configure Codex CLI automatically. Or configure it manually:

```bash
export CODEX_API_KEY="sk-your-api-key-here"
```

And add the following to `~/.codex/config.toml`:

```toml
model = "gpt-4o"
model_provider = "chat2responses"

[model_providers.chat2responses]
base_url = "http://127.0.0.1:8000/v1"
env_key = "CODEX_API_KEY"
wire_api = "responses"
```

Now run `codex` — it will use the proxy, translating Responses API calls to Chat Completions upstream.

## Usage

```
chat2responses           Start proxy server (auto-detect config)
chat2responses setup     Interactive setup wizard
chat2responses serve     Start proxy server (requires config.json)
chat2responses version   Show version
chat2responses help      Show this help
```

Configuration is loaded from the first available location:
1. `./config.json`
2. `$XDG_CONFIG_HOME/chat2responses/config.json`
3. `~/.config/chat2responses/config.json`
4. `/etc/chat2responses/config.json`

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/responses` | POST | Proxy endpoint — accepts Responses API format, forwards as Chat Completions |
| `/v1/models` | GET | Pass-through to upstream model list |
| `/health` | GET | Health check |

## Architecture

```
┌─────────────┐     Responses API      ┌──────────────────┐     Chat Completions      ┌──────────────┐
│  App/Client  │ ──────────────────►   │  chat2responses  │ ──────────────────►   │   Upstream   │
│  (Codex CLI) │ ◄──────────────────   │  (Proxy Server)  │ ◄──────────────────   │  (OpenAI,    │
└─────────────┘     Responses API      └──────────────────┘     Chat Completions      │   Azure,     │
                                                                                   │   Local LLM) │
                                                                                   └──────────────┘
```

The proxy performs the following translations:

- **Request**: `instructions` → `system` message, `input` items → `messages` array, `tools` → Chat Completions tool format, `max_output_tokens` → `max_tokens`
- **Response** (non-streaming): Chat `choices[0].message` → Responses `output` items (text + function calls), `usage` tokens remapped
- **Streaming**: Chat SSE chunks → Responses SSE events (`response.output_text.delta`, `response.function_call_arguments.delta`, etc.)

## Build

```bash
make build              # Current platform
make build-all          # All platforms (Linux, macOS Intel/ARM, Windows)
make fmt                # Format code
make vet                # Run go vet
make test               # Run tests
make clean              # Clean build artifacts
```

Prebuilt binaries are output to the `release/` directory.

## License

MIT

---

<a id="chinese"></a>

# chat2responses

> **Chat Completions → Responses API 代理**

一个轻量、零依赖的代理服务器，将 OpenAI **Chat Completions API** 请求转换为 **Responses API** 格式。兼容任何 OpenAI 兼容的上游服务，包括 OpenAI、Azure OpenAI 以及本地 LLM 服务器。

---

## 概述

`chat2responses` 位于你的应用与任何 OpenAI 兼容的 Chat Completions 端点之间。它暴露一个 **Responses API** 端点（`POST /v1/responses`），并实时转换请求——在两种 API 格式之间转换输入项、工具、流式事件和函数调用。

主要为 [Codex CLI](https://github.com/openai/codex) 而构建，Codex CLI 原生使用 Responses API。将 Codex 指向 `chat2responses`，它就可以无修改地与任何 Chat Completions 上游通信。

## 特性

- **协议转换**：Responses API ↔ Chat Completions API — 请求与响应的双向映射
- **会话隔离**：每个 `previous_response_id` 链独立追踪，允许多个客户端（如 Codex CLI + Codex Desktop）同时维持独立的对话
- **流式支持**：实时 SSE 转换（文本增量、推理内容、函数调用参数）
- **工具/函数调用处理**：正确将 Responses API 的工具和 tool_choice 映射为 Chat Completions 格式，并转换回函数调用
- **多模态输入**：将 `input_text` 和 `input_image` 内容块转换为聊天消息
- **Codex CLI 集成**：交互式设置向导，自动配置 Codex 使用代理
- **多平台二进制**：预编译 Linux、macOS（Intel + Apple Silicon）和 Windows 版本
- **Docker 支持**：基于 scratch 的最小化镜像

## 安装

### 下载预编译版本

从 [releases 页面](https://github.com/fooyii/chat2responses/releases) 获取适合你平台的最新二进制文件。

```bash
# macOS (Apple Silicon)
curl -LO https://github.com/fooyii/chat2responses/releases/latest/download/chat2responses-darwin-arm64
chmod +x chat2responses-darwin-arm64
sudo mv chat2responses-darwin-arm64 /usr/local/bin/chat2responses

# macOS (Intel)
curl -LO https://github.com/fooyii/chat2responses/releases/latest/download/chat2responses-darwin-amd64
chmod +x chat2responses-darwin-amd64
sudo mv chat2responses-darwin-amd64 /usr/local/bin/chat2responses

# Linux
curl -LO https://github.com/fooyii/chat2responses/releases/latest/download/chat2responses-linux-amd64
chmod +x chat2responses-linux-amd64
sudo mv chat2responses-linux-amd64 /usr/local/bin/chat2responses
```

### 从源码构建

需要 Go 1.21+。

```bash
git clone https://github.com/fooyii/chat2responses.git
cd chat2responses
make build
```

### Docker

```bash
docker build -t chat2responses .
docker run -p 8000:8000 -v $(pwd)/config.json:/config.json chat2responses
```

## 快速开始

### 1. 配置

创建 `config.json`：

```json
{
  "upstream": {
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-your-api-key-here"
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8000
  },
  "model": {
    "default_model": "gpt-4o"
  }
}
```

或者运行交互式设置向导：

```bash
./chat2responses setup
```

### 2. 启动服务

```bash
./chat2responses serve
```

代理服务启动在 `http://0.0.0.0:8000`。

### 3. 使用

发送 Responses API 请求：

```bash
curl http://localhost:8000/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "input": "What is the capital of France?",
    "instructions": "Answer concisely."
  }'
```

流式请求：

```bash
curl http://localhost:8000/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "input": "Count from 1 to 5.",
    "stream": true
  }'
```

### 4. 配合 Codex CLI 使用

设置向导可以自动配置 Codex CLI。也可以手动配置：

```bash
export CODEX_API_KEY="sk-your-api-key-here"
```

然后将以下内容添加到 `~/.codex/config.toml`：

```toml
model = "gpt-4o"
model_provider = "chat2responses"

[model_providers.chat2responses]
base_url = "http://127.0.0.1:8000/v1"
env_key = "CODEX_API_KEY"
wire_api = "responses"
```

现在运行 `codex`——它将通过代理，将 Responses API 调用转换为 Chat Completions 上游请求。

## 命令

```
chat2responses           启动代理服务器（自动检测配置）
chat2responses setup     交互式设置向导
chat2responses serve     启动代理服务器（需要 config.json）
chat2responses version   显示版本号
chat2responses help      显示帮助信息
```

配置文件按以下顺序加载：
1. `./config.json`
2. `$XDG_CONFIG_HOME/chat2responses/config.json`
3. `~/.config/chat2responses/config.json`
4. `/etc/chat2responses/config.json`

## API 端点

| 端点 | 方法 | 描述 |
|----------|--------|------|
| `/v1/responses` | POST | 代理端点 — 接收 Responses API 格式，转发为 Chat Completions |
| `/v1/models` | GET | 透传到上游的模型列表 |
| `/health` | GET | 健康检查 |

## 架构

```
┌─────────────┐     Responses API      ┌──────────────────┐     Chat Completions      ┌──────────────┐
│  应用/客户端  │ ──────────────────►   │  chat2responses  │ ──────────────────►   │   上游服务    │
│  (Codex CLI) │ ◄──────────────────   │   (代理服务器)    │ ◄──────────────────   │  (OpenAI,    │
└─────────────┘     Responses API      └──────────────────┘     Chat Completions      │   Azure,     │
                                                                                   │   本地 LLM)  │
                                                                                   └──────────────┘
```

代理执行以下转换：

- **请求**：`instructions` → `system` 消息，`input` 项 → `messages` 数组，`tools` → Chat Completions 工具格式，`max_output_tokens` → `max_tokens`
- **响应**（非流式）：Chat `choices[0].message` → Responses `output` 项（文本 + 函数调用），`usage` tokens 重新映射
- **流式**：Chat SSE 块 → Responses SSE 事件（`response.output_text.delta`、`response.function_call_arguments.delta` 等）

## 构建

```bash
make build              # 当前平台
make build-all          # 所有平台 (Linux, macOS Intel/ARM, Windows)
make fmt                # 格式化代码
make vet                # 运行 go vet
make test               # 运行测试
make clean              # 清理构建产物
```

预编译二进制文件输出到 `release/` 目录。

## 许可证

MIT
