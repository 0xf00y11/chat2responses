# chat2responses

> Chat Completions → Responses API 协议转换代理

**chat2responses** 是一个轻量级 HTTP 代理服务器，能够将 **Chat Completions API**（`/v1/chat/completions`）的请求和响应实时转换为 **Responses API**（`/v1/responses`）格式。使得依赖 Responses API 的工具（如 [OpenAI Codex CLI](https://github.com/openai/codex)）能够无缝对接任何兼容 Chat Completions 的 API 提供商。

---

## ✨ 功能特性

- **协议转换** — 自动将 Requests API 请求映射为 Chat Completions 格式，并将响应转换回 Responses API 格式
- **流式支持** — 原生支持 Server-Sent Events (SSE) 流式响应，低延迟逐 token 转发
- **函数调用** — 完整支持 `function_call` 与 `function_call_output` 的双向转换
- **工具定义** — 透传工具定义（tools），支持并行工具调用
- **模型列表** — 代理 `/v1/models` 端点，透明返回上游可用模型
- **交互式向导** — 提供友好的命令行设置向导，一键完成配置
- **Codex CLI 集成** — 自动配置 Codex CLI，开箱即用
- **多源配置** — 支持 `config.json`、`XDG_CONFIG_HOME`、`/etc/chat2responses/config.json` 多层配置加载
- **跨平台** — 支持 Linux、macOS（Intel + Apple Silicon）、Windows

## 🚀 快速开始

### 下载

从 [Releases](https://github.com/fooyii/chat2responses/releases) 页面下载对应平台的二进制文件：

| 平台 | 文件 |
|------|------|
| Linux x86_64 | `chat2responses-linux-amd64` |
| macOS Intel | `chat2responses-darwin-amd64` |
| macOS Apple Silicon | `chat2responses-darwin-arm64` |
| Windows x86_64 | `chat2responses-windows-amd64.exe` |

### 运行

```bash
# 直接运行，进入交互式设置向导
./chat2responses
```

向导会引导你完成以下步骤：

1. 输入上游 API 地址、API Key 和模型名称
2. 选择是否配置到 Codex CLI
3. 选择是否启动代理服务器

### 直接启动服务

```bash
# 提前准备好 config.json，直接启动代理
./chat2responses serve
```

## ⚙️ 配置

### 配置文件 `config.json`

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

| 字段 | 类型 | 说明 |
|------|------|------|
| `upstream.base_url` | string | 上游 Chat Completions API 地址 |
| `upstream.api_key` | string | API 密钥 |
| `server.host` | string | 代理服务监听地址（默认 `0.0.0.0`）|
| `server.port` | int | 代理服务监听端口（默认 `8000`）|
| `model.default_model` | string | 默认模型名称 |
| `debug` | bool | 是否启用调试模式 |

### 配置加载优先级

1. 当前目录 `./config.json`
2. `$XDG_CONFIG_HOME/chat2responses/config.json`
3. `$HOME/.config/chat2responses/config.json`
4. `/etc/chat2responses/config.json`

## 📡 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/responses` | POST | Responses API 代理入口 |
| `/v1/models` | GET | 获取上游可用模型列表 |
| `/health` | GET | 健康检查 |

### 协议映射

| Responses API | Chat Completions |
|---------------|------------------|
| `instructions` → | System message |
| `input` (string) → | User message |
| `input` (array) → | 多条消息（user / assistant / tool）|
| `function_call` → | Assistant `tool_calls` |
| `function_call_output` → | Tool message |
| `developer` role → | `system` role |
| `max_output_tokens` → | `max_tokens` |
| ← `message.content` | `output[].content[].text` ← |
| ← `tool_calls[]` | `output[].function_call` ← |

## 🔧 与 Codex CLI 集成

运行设置向导后，按照提示即可自动配置 Codex CLI。也可以通过环境变量手动配置：

```bash
export CODEX_API_KEY="your-api-key"
export CODEX_BASE_URL="http://127.0.0.1:8000/v1"
codex
```

如需恢复原始的 Codex CLI 配置：

```bash
./chat2responses restore
```

## 🏗️ 自行编译

### 前提条件

- Go 1.26+（建议使用最新版本）
- `make` 工具（任意实现）

### 编译当前平台

```bash
git clone https://github.com/fooyii/chat2responses.git
cd chat2responses
make build
```

### 交叉编译所有平台

```bash
make build-all
```

编译产物位于 `release/` 目录：

```
release/
├── chat2responses-linux-amd64
├── chat2responses-darwin-amd64
├── chat2responses-darwin-arm64
└── chat2responses-windows-amd64.exe
```

## 🏗️ 项目结构

```
chat2responses/
├── cmd/
│   └── chat2responses/    # 主程序入口与交互式向导
├── internal/
│   ├── codex/             # Codex CLI 配置管理
│   ├── config/            # 配置加载与保存
│   ├── model/             # Chat Completions & Responses API 数据结构
│   ├── proxy/             # 上游客户端、协议转换与流式处理
│   └── server/            # HTTP 服务端
├── Makefile               # 构建与交叉编译
├── config.json            # 配置文件
├── config.json.example    # 配置示例
├── Dockerfile             # Docker 构建文件
└── go.mod                 # Go 模块定义
```

## 🐳 Docker

```bash
docker build -t chat2responses .
docker run -d -p 8000:8000 -v $(pwd)/config.json:/app/config.json chat2responses
```

## 📄 许可证

Copyright (c) 2026 fooyii.

## 👤 作者

**fooyii** — [GitHub](https://github.com/fooyii)

---

*让 Chat Completions 生态无缝接入 Responses API 时代*
