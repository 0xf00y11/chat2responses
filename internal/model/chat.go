// Package model - chat2responses 数据模型定义 - 提供 Chat Completions 和 Responses API 的数据结构
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package model

import "encoding/json"

type ChatRequest struct {
	Model             string        `json:"model"`
	Messages          []ChatMessage `json:"messages"`
	Stream            bool          `json:"stream,omitempty"`
	MaxTokens         int           `json:"max_tokens,omitempty"`
	Temperature       *float64      `json:"temperature,omitempty"`
	TopP              *float64      `json:"top_p,omitempty"`
	Tools             []ChatTool    `json:"tools,omitempty"`
	ParallelToolCalls *bool         `json:"parallel_tool_calls,omitempty"`
	ToolChoice        interface{}   `json:"tool_choice,omitempty"`
}

type ChatMessage struct {
	Role             string         `json:"role"`
	Content          interface{}    `json:"content"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ToolCalls        []ChatToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
}

type ChatToolCall struct {
	ID               string       `json:"id"`
	Type             string       `json:"type"`
	Function         ChatFunction `json:"function"`
	ThoughtSignature string       `json:"thought_signature,omitempty"`
}

func (c ChatToolCall) MarshalJSON() ([]byte, error) {
	type Alias ChatToolCall
	if c.ThoughtSignature == "" {
		return json.Marshal(Alias(c))
	}
	var aux struct {
		Alias
		ExtraContent map[string]interface{} `json:"extra_content"`
	}
	aux.Alias = Alias(c)
	aux.ExtraContent = map[string]interface{}{
		"google": map[string]interface{}{
			"thought_signature": c.ThoughtSignature,
		},
	}
	return json.Marshal(aux)
}

func (c *ChatToolCall) UnmarshalJSON(data []byte) error {
	type Alias ChatToolCall
	var aux struct {
		Alias
		ExtraContent      map[string]interface{} `json:"extra_content"`
		ExtraContentCamel map[string]interface{} `json:"extraContent"`
		Google            map[string]interface{} `json:"google"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*c = ChatToolCall(aux.Alias)

	extractSig := func(m map[string]interface{}) string {
		if m == nil {
			return ""
		}
		if google, ok := m["google"].(map[string]interface{}); ok {
			if ts, ok := google["thought_signature"].(string); ok && ts != "" {
				return ts
			}
			if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
				return ts
			}
		}
		return ""
	}

	if c.ThoughtSignature == "" {
		if ts := extractSig(aux.ExtraContent); ts != "" {
			c.ThoughtSignature = ts
		} else if ts := extractSig(aux.ExtraContentCamel); ts != "" {
			c.ThoughtSignature = ts
		} else if google, ok := aux.Google["google"].(map[string]interface{}); ok {
			if ts, ok := google["thought_signature"].(string); ok && ts != "" {
				c.ThoughtSignature = ts
			}
		} else if ts, ok := aux.Google["thought_signature"].(string); ok && ts != "" {
			c.ThoughtSignature = ts
		}
	}
	return nil
}

type ChatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatTool struct {
	Type     string            `json:"type"`
	Function *ChatToolFunction `json:"function,omitempty"`
}

type ChatToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatStreamChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []ChatStreamChoice `json:"choices"`
	Usage   *Usage             `json:"usage,omitempty"`
}

type ChatStreamChoice struct {
	Index        int       `json:"index"`
	Delta        ChatDelta `json:"delta"`
	FinishReason string    `json:"finish_reason,omitempty"`
}

type ChatDelta struct {
	Role             string         `json:"role,omitempty"`
	Content          string         `json:"content,omitempty"`
	ToolCalls        []ChatToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
}
