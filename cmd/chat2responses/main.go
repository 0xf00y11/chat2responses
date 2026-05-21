// cmd/chat2responses - chat2responses entry point
//
// Copyright (c) 2025 fooyii. MIT License.

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"chat2responses/internal/codex"
	"chat2responses/internal/config"
	"chat2responses/internal/server"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			cfg, err := config.Load("")
			if err != nil {
				fmt.Fprintf(os.Stderr, "config error: %s\n", err)
				os.Exit(1)
			}
			server.Run(cfg)
			return
		case "version", "-v", "--version":
			fmt.Printf("chat2responses v%s\n", version)
			return
		case "help", "-h", "--help":
			printUsage()
			return
		}
	}

	interactiveSetup()
}

func printUsage() {
	fmt.Println(`chat2responses - Chat Completions to Responses API proxy

USAGE:
    chat2responses           Interactive setup wizard
    chat2responses serve     Start proxy server (requires config.json)
    chat2responses version   Show version
    chat2responses help      Show this help`)
}

func interactiveSetup() {
	fmt.Println("╔═════════════════════════════════════╗")
	fmt.Println("║     chat2responses 设置向导 v0.1.0    ║")
	fmt.Println("║   Chat Completions → Responses API    ║")
	fmt.Println("╚═════════════════════════════════════╝")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	cfg := config.DefaultConfig

	fmt.Print("上游 API 地址 (如 https://api.openai.com/v1): ")
	input, _ := reader.ReadString('\n')
	cfg.Upstream.BaseURL = strings.TrimSpace(input)

	fmt.Print("API Key: ")
	input, _ = reader.ReadString('\n')
	cfg.Upstream.APIKey = strings.TrimSpace(input)

	defaultModel := cfg.Model.DefaultModel
	if defaultModel == "" {
		defaultModel = "gpt-4o"
	}
	fmt.Printf("模型 (默认 %s): ", defaultModel)
	input, _ = reader.ReadString('\n')
	if m := strings.TrimSpace(input); m != "" {
		cfg.Model.DefaultModel = m
	} else {
		cfg.Model.DefaultModel = defaultModel
	}

	fmt.Println()
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Printf("│  地址: %-30s │\n", cfg.Upstream.BaseURL)
	fmt.Printf("│  密钥: %-30s │\n", maskKey(cfg.Upstream.APIKey))
	fmt.Printf("│  模型: %-30s │\n", cfg.Model.DefaultModel)
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	if err := config.Save(&cfg, "config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "保存配置失败: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ 配置已保存到 config.json")
	fmt.Println()

	fmt.Print("是否配置到 Codex CLI? (Y/n): ")
	input, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(input)) != "n" {
		setupCfg := codex.SetupConfig{
			Model:   cfg.Model.DefaultModel,
			BaseURL: fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port),
			APIKey:  cfg.Upstream.APIKey,
		}
		if err := codex.Setup(setupCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Codex 配置失败: %s\n", err)
		} else {
			fmt.Println()
			fmt.Println("已设置 CODEX_API_KEY 环境变量，请执行:")
			fmt.Printf("  export CODEX_API_KEY=\"%s\"\n", cfg.Upstream.APIKey)
			fmt.Println("然后运行: codex")
		}
	}

	fmt.Println()
	fmt.Print("是否启动代理服务器? (Y/n): ")
	input, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(input)) != "n" {
		fmt.Println()
		fmt.Printf("🚀 启动代理服务器于 http://127.0.0.1:%d...\n", cfg.Server.Port)
		server.Run(&cfg)
	}

	fmt.Println("配置完成, 按 Ctrl+C 退出")
	select {}
}

func maskKey(key string) string {
	if len(key) <= 6 {
		return "******"
	}
	return key[:3] + "..." + key[len(key)-3:]
}
