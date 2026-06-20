// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-20
// Package proxy - 上游 API 客户端 - stream 流式转换的辅助工具与数据提取
// Copyright (c) 2026 fooyii.
// Created: 2026-06-20

package proxy

import (
	"strings"
)

type toolCallBuilder struct {
	Index            int
	ID               string
	Name             string
	Args             strings.Builder
	ThoughtSignature string
}

// extractThoughtSignature - 从上游 Tool Call Delta 中多级提取并归一化思维链签名（Thought Signature）
func extractThoughtSignature(tc map[string]interface{}) string {
	if ts, ok := tc["thought_signature"].(string); ok && ts != "" {
		return ts
	}
	if ts, ok := tc["thoughtSignature"].(string); ok && ts != "" {
		return ts
	}
	if ec, ok := tc["extra_content"].(map[string]interface{}); ok {
		if google, ok := ec["google"].(map[string]interface{}); ok {
			if ts, ok := google["thought_signature"].(string); ok && ts != "" {
				return ts
			}
			if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
				return ts
			}
		}
	}
	if ec, ok := tc["extraContent"].(map[string]interface{}); ok {
		if google, ok := ec["google"].(map[string]interface{}); ok {
			if ts, ok := google["thought_signature"].(string); ok && ts != "" {
				return ts
			}
			if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
				return ts
			}
		}
	}
	if google, ok := tc["google"].(map[string]interface{}); ok {
		if ts, ok := google["thought_signature"].(string); ok && ts != "" {
			return ts
		}
		if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
			return ts
		}
	}
	if fn, ok := tc["function"].(map[string]interface{}); ok {
		if ts, ok := fn["thought_signature"].(string); ok && ts != "" {
			return ts
		}
		if ts, ok := fn["thoughtSignature"].(string); ok && ts != "" {
			return ts
		}
	}
	return ""
}

// buildUsage - 解析并归一化 Token Usage 结构体
func buildUsage(u map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"input_tokens":  0,
		"output_tokens": 0,
		"total_tokens":  0,
	}
	if u == nil {
		return result
	}
	if v, ok := u["prompt_tokens"]; ok {
		result["input_tokens"] = v
	}
	if v, ok := u["completion_tokens"]; ok {
		result["output_tokens"] = v
	}
	if v, ok := u["total_tokens"]; ok {
		result["total_tokens"] = v
	}
	return result
}
