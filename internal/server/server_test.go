// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-13
package server

import (
	"testing"

	"chat2responses/internal/config"
	"chat2responses/internal/model"
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

func TestParseModelCommand(t *testing.T) {
	// 1. 测试标准单个字符串 Content 类型的 /model 列表指令
	msgs1 := []model.ChatMessage{
		{Role: "user", Content: "/model"},
	}
	cmdType, param, isCmd := parseModelCommand(msgs1)
	if !isCmd || cmdType != "list" || param != "" {
		t.Errorf("Expected list command, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}

	// 2. 测试带参数的 /model 切换指令
	msgs2 := []model.ChatMessage{
		{Role: "user", Content: "/model deepseek-v4-flash"},
	}
	cmdType, param, isCmd = parseModelCommand(msgs2)
	if !isCmd || cmdType != "switch" || param != "deepseek-v4-flash" {
		t.Errorf("Expected switch command, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}

	// 3. 测试 Content 为 []interface{}（富文本数组/零件）模式下的 /model 拦截（兼容部分高级客户端）
	msgs3 := []model.ChatMessage{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "input_text",
					"text": "/model  ",
				},
			},
		},
	}
	cmdType, param, isCmd = parseModelCommand(msgs3)
	if !isCmd || cmdType != "list" || param != "" {
		t.Errorf("Expected list command from slice content, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}

	// 4. 测试非用户消息
	msgs4 := []model.ChatMessage{
		{Role: "assistant", Content: "/model"},
	}
	_, _, isCmd = parseModelCommand(msgs4)
	if isCmd {
		t.Errorf("Assistant messages should not trigger commands")
	}

	// 5. 测试 /switch 指令
	msgs5 := []model.ChatMessage{
		{Role: "user", Content: "/switch"},
	}
	cmdType, param, isCmd = parseModelCommand(msgs5)
	if !isCmd || cmdType != "list" || param != "" {
		t.Errorf("Expected list command from /switch, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}

	// 6. 测试 /use <model> 指令
	msgs6 := []model.ChatMessage{
		{Role: "user", Content: "/use deepseek-v4-pro"},
	}
	cmdType, param, isCmd = parseModelCommand(msgs6)
	if !isCmd || cmdType != "switch" || param != "deepseek-v4-pro" {
		t.Errorf("Expected switch command from /use, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}
	// 7. 测试 !switch 列表指令
	msgs7 := []model.ChatMessage{
		{Role: "user", Content: "!switch"},
	}
	cmdType, param, isCmd = parseModelCommand(msgs7)
	if !isCmd || cmdType != "list" || param != "" {
		t.Errorf("Expected list command from !switch, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}

	// 8. 测试 #use <model> 切换指令
	msgs8 := []model.ChatMessage{
		{Role: "user", Content: "#use gemini-3.5-flash"},
	}
	cmdType, param, isCmd = parseModelCommand(msgs8)
	if !isCmd || cmdType != "switch" || param != "gemini-3.5-flash" {
		t.Errorf("Expected switch command from #use, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}

	// 9. 测试 :switch <model> 切换指令
	msgs9 := []model.ChatMessage{
		{Role: "user", Content: ":switch glm-5.1"},
	}
	cmdType, param, isCmd = parseModelCommand(msgs9)
	if !isCmd || cmdType != "switch" || param != "glm-5.1" {
		t.Errorf("Expected switch command from :switch, got isCmd=%v, cmdType=%s, param=%s", isCmd, cmdType, param)
	}
}
