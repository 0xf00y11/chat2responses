// Package model - chat2responses 数据模型定义 - 提供 Chat Completions 和 Responses API 的数据结构
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package model

import (
	"encoding/json"
	"fmt"
	"time"
)

type ResponsesRequest struct {
	Model              string           `json:"model"`
	Input              json.RawMessage  `json:"input"`
	Instructions        string           `json:"instructions,omitempty"`
	MaxOutputTokens    int              `json:"max_output_tokens,omitempty"`
	Stream             bool             `json:"stream,omitempty"`
	Temperature        *float64         `json:"temperature,omitempty"`
	TopP               *float64         `json:"top_p,omitempty"`
	Tools              []ResponseTool   `json:"tools,omitempty"`
	ToolChoice         interface{}      `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool            `json:"parallel_tool_calls,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	Store              *bool             `json:"store,omitempty"`
	PreviousResponseID string           `json:"previous_response_id,omitempty"`
}

type ResponseTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type ResponsesResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created_at"`
	Status  string              `json:"status"`
	Model   string              `json:"model"`
	Output  []ResponseOutputItem `json:"output"`
	Usage   *Usage              `json:"usage,omitempty"`
}

type ResponseOutputItem struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Role      string         `json:"role,omitempty"`
	Status    string         `json:"status,omitempty"`
	Content   []ContentBlock `json:"content,omitempty"`
	CallID    string         `json:"call_id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments string         `json:"arguments,omitempty"`
}

type ContentBlock struct {
	Type        string        `json:"type"`
	Text        string        `json:"text,omitempty"`
	Annotations []interface{} `json:"annotations,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func MakeID(prefix ...string) string {
	p := "resp"
	if len(prefix) > 0 {
		p = prefix[0]
	}
	return fmt.Sprintf("%s_%x", p, time.Now().UnixNano())
}

