# chat2responses

> Chat Completions → Responses API 协议转换代理

---

## 背景

OpenAI Codex CLI 使用 Responses API（`/v1/responses`）作为标准接口。然而目前大多数模型提供商（包括部署在私服、开源模型等）仅支持传统的 Chat Completions API（`/v1/chat/completions`）。**chat2responses** 就是为了填补这个断层而生——一个透明代理，在两种协议之间实时转换，让 Codex CLI 能接入任意兼容 Chat Completions 的 API。

---

## 两种协议的区别

| 维度 | Chat Completions API | Responses API |
|---------|---------------------|---------------|
| 端点 | `/v1/chat/completions` | `/v1/responses` |
| 消息结构 | `messages[]` 数组 | `input` + `instructions` |
| 官方角色 | `system`、`user`、`assistant`、`tool` | `user`、`assistant`、`developer`、`tool` |
| 工具调用 | `tool_calls`（负责人帮助人） | `function_call` + `function_call_output` |
| 输出结构 | `choices[].message` | `output[]` 数组 |
| 指令 | 无独立字段，放入 `system` 消息 | `instructions` 字段独立传递 |
| 流式 | SSE `data: {"choices":[...]}` | SSE `data: {"type": "response.output_text.delta"}` |

---
## 工作原理

代理处于 Codex CLI 和上游 API 之间：

```
Codex CLI                           上游 API
   │                                   │
   │  POST /v1/responses               │
   │  (Responses API 格式)            │
   │─────────────────▒─────▒               │
   │                             │               │
   │                    ┌───────┐       │
   │                    │ 转换器 │       │
   │                    ┌───────┬       │
   │                             │               │
   │                             ▖────────────▒
   │                             POST /v1/chat/completions
   │                             (Chat Completions 格式)
   │                                   │
```

具体转换逻辑参见下文的协议映射表。

---

## 功能特性

- **协议转换** — 将 Responses API 请求映射为 Chat Completions 格式，并将响应转换回 Responses API 格式
- **流式支持** — 原生支持 SSE 流式响应，逐 token 转发
- **函数调用** — 支持 `function_call` 与 `function_call_output` 双向转换
- **工具定义** — 透传工具定义，支持并行工具调用
- **模型列表** — 代理 `/v1/models` 端点返回上游可用模型
- **交互式向导** — 命令行设置向导，一键配置
- **Codex CLI 集成** — 自动配置 Codex CLI
- **多源配置** — 支持多层配置加载
- **跨平台** — Linux、macOS（Intel + Apple Silicon）、Windows

---

## 快速开始

### 下载

从 [Releases](https://github.com/0xf00y11/chat2responses/releases) 页面下载对应平台的二进制文件：

| 平台 | 文件 |
|------|------|
| Linux x86_64 | `chat2responses-linux-amd64` |
| macOS Intel | `chat2responses-darwin-amd64` |
| macOS Apple Silicon | `chat2responses-darwin-arm64` |
| Windows x86_64 | `chat2responses-windows-amd64.exe` |

### 运行

```bash
# 交互式设置向导
./chat2responses

# 直接启动代理（需先准备好 config.json）
./chat2responses serve
```

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
|------|------|--------|------|
| `upstream.base_url` | string | — | 上游 Chat Completions API 地址 |
| `upstream.api_key` | string | — | API 密钥 |
| `server.host` | string | `"0.0.0.0"` | 监听地址 |
| `server.port` | int | `8000` | 监听端口 |
| `model.default_model` | string | — | 默认模型名称 |
| `debug` | bool | `false` | 调试模式 |

### 配置加载优先级

1. 当前目录 `./config.json`
2. `$XDG_CONFIG_HOME/chat2responses/config.json`（未设置时回退 `$HOME/.config/`）
3. `/etc/chat2responses/config.json`

---

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/responses` | POST | Responses API 代理入口 |
| `/v1/models` | GET | 上游模型列表 |
| `/health` | GET | 健康检查 |

### 协议映射

| Responses API → | Chat Completions |
|----------------|------------------|
| `instructions` | System message |
| `input`（字符串）| User message |
| `input`（数组）| 多条消息 |
| `function_call` | Assistant `tool_calls` |
| `function_call_output` | Tool message |
| `role: developer` | `role: system` |
| `max_output_tokens` | `max_tokens` |

| ← Chat Completions | Responses API |
|--------------------|---------------|
| `choices[].message.content` | `output[].content[].text` |
| `choices[].message.tool_calls` | `output[].function_call` |

---

## Codex CLI 集成

运行设置向导后按提示操作即可。也可通过环境变量手动配置：

```bash
export CODEX_API_KEY="your-api-key"
export CODEX_BASE_URL="http://127.0.0.1:8000/v1"
codex
```

恢复原始配置：

```bash
./chat2responses restore
```

---

## 自行编译

### 前置要求

- Go 1.26+
- make

### 编译

```bash
git clone https://github.com/0xf00y11/chat2responses.git
cd chat2responses
make build
```

交叉编译所有平台：

```bash
make build-all
```

编译产物位于 `release/`：

```
release/
├── chat2responses-linux-amd64
├── chat2responses-darwin-amd64
├── chat2responses-darwin-arm64
└── chat2responses-windows-amd64.exe
```

---

## 项目结构

```
chat2responses/
├── cmd/chat2responses/    # 主入口与交互式向导
├── internal/
│   ├── codex/             # Codex CLI 配置管理
│   ├── config/            # 配置加载
│   ├── model/             # API 数据结构
│   ├── proxy/             # 协议转换与流式处理
│   └── server/            # HTTP 服务端
├── Makefile               # 构建与交叉编译
├── Dockerfile
├── go.mod
├── config.json.example
└── README.md
```

---

## Docker

```bash
docker build -t chat2responses .
docker run -d -p 8000:8000 -v $(pwd)/config.json:/config.json chat2responses
```

---

Copyright (c) 2026 fooyii.

---

## 作者

**fooyii** — [GitHub](https://github.com/0xf00y11)
