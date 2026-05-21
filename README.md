# chat2responses

> Chat Completions → Responses API proxy for OpenAI Codex CLI

A lightweight proxy that converts **Chat Completions API** (`/v1/chat/completions`) 
requests and responses into **Responses API** (`/v1/responses`) format — enabling 
tools like [OpenAI Codex CLI](https://github.com/openai/codex) (which use the 
Responses API) to work with **any** Chat Completions compatible API provider.

## Quick Start

```bash
# Run the interactive setup wizard
./chat2responses
```

You'll be guided through:
1. Enter API address, API key, and model
2. Choose whether to configure Codex CLI
3. Choose whether to start the proxy server

## Protocol

| Responses API | Chat Completions |
|--------------|-----------------|
| `instructions` → | System message |
| `input` (string) → | User message |
| `function_call` → | Assistant `tool_calls` |
| `function_call_output` → | Tool message |
| `developer` role → | `system` role |
| `max_output_tokens` → | `max_tokens` |
| ← `message.content` | `output[].content[].text` |
| ← `tool_calls[]` | `output[].function_call` |

## License

MIT
