// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-13
package server

import (
	"testing"

	"chat2responses/internal/config"
)

func TestGetClientForModel(t *testing.T) {
	cfg := &config.Config{
		Upstream: config.UpstreamConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-default",
		},
		Model: config.ModelConfig{
			DefaultModel: "gpt-4o",
		},
		Models: map[string]config.ModelUpstream{
			"deepseek-v4-flash": {
				BaseURL: "https://api.deepseek.com/v1",
				APIKey:  "sk-deepseek-flash",
			},
			"deepseek-v4-pro": {
				BaseURL: "https://api.deepseek.com/v1",
				APIKey:  "sk-deepseek-pro",
			},
			"gemini-3.5-flash": {
				BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
				APIKey:  "sk-gemini",
			},
			"fast-gpt": {
				UpstreamModel: "gpt-4o-mini", // 别名模型测试
			},
		},
	}

	s := New(cfg)

	// 1. 验证默认模型是否回退至默认 client，且模型名不改变
	defaultClient, defaultModel := s.getClientForModel("gpt-4o")
	if defaultClient != s.client || defaultModel != "gpt-4o" {
		t.Errorf("expected default client & gpt-4o, got %v & %s", defaultClient, defaultModel)
	}

	// 2. 验证 deepseek-v4-flash 路由
	flashClient, flashModel := s.getClientForModel("deepseek-v4-flash")
	expectedClient := s.clients["deepseek-v4-flash"]
	if flashClient != expectedClient || flashModel != "deepseek-v4-flash" {
		t.Errorf("expected flash client & deepseek-v4-flash, got %v & %s", flashClient, flashModel)
	}

	// 3. 验证 deepseek-v4-pro 路由
	proClient, proModel := s.getClientForModel("deepseek-v4-pro")
	expectedProClient := s.clients["deepseek-v4-pro"]
	if proClient != expectedProClient || proModel != "deepseek-v4-pro" {
		t.Errorf("expected pro client & deepseek-v4-pro, got %v & %s", proClient, proModel)
	}

	// 4. 验证 gemini-3.5-flash 路由
	geminiClient, geminiModel := s.getClientForModel("gemini-3.5-flash")
	expectedGeminiClient := s.clients["gemini-3.5-flash"]
	if geminiClient != expectedGeminiClient || geminiModel != "gemini-3.5-flash" {
		t.Errorf("expected gemini client & gemini-3.5-flash, got %v & %s", geminiClient, geminiModel)
	}

	// 5. 验证 fast-gpt 别名路由（只配置了别名，未配置专属上游，应使用默认上游并重映射模型名）
	fastClient, fastModel := s.getClientForModel("fast-gpt")
	if fastClient != s.client || fastModel != "gpt-4o-mini" {
		t.Errorf("expected default client & gpt-4o-mini, got %v & %s", fastClient, fastModel)
	}

	// 6. 验证空模型名时的安全回退
	fallbackClient, fallbackModel := s.getClientForModel("")
	if fallbackClient != s.client || fallbackModel != "gpt-4o" {
		t.Errorf("expected default client & gpt-4o for empty model name, got %v & %s", fallbackClient, fallbackModel)
	}
}
