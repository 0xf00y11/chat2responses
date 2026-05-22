// cmd/chat2responses - chat2responses 主程序入口 - 交互式设置向导与代理服务器
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

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
		case "setup":
			interactiveSetup()
			return
		case "version", "-v", "--version":
			fmt.Printf("chat2responses v%s\n", version)
			return
		case "help", "-h", "--help":
			printUsage()
			return
		}
	}

	// 已有有效配置则直接启动服务，免交互
	if cfg, err := config.Load(""); err == nil {
		fmt.Println("\u2713 检测到已有配置，跳过设置向导")
		fmt.Printf("\U0001f680 启动代理服务器于 http://127.0.0.1:%d...\n", cfg.Server.Port)
		server.Run(cfg)
		return
	}

	interactiveSetup()
}

func printUsage() {
	fmt.Println(`chat2responses - Chat Completions to Responses API proxy

USAGE:
    chat2responses           Start proxy server (auto-detect config)
    chat2responses setup     Interactive setup wizard
    chat2responses serve     Start proxy server (requires config.json)
    chat2responses version   Show version
    chat2responses help      Show this help`)
}

func interactiveSetup() {
	fmt.Println("\u2554" + strings.Repeat("\u2550", 39) + "\u2557")
	fmt.Println("\u2551" + "     chat2responses \u8bbe\u7f6e\u5411\u5bfc v0.1.0    " + "\u2551")
	fmt.Println("\u2551" + "   Chat Completions \u2192 Responses API    " + "\u2551")
	fmt.Println("\u255a" + strings.Repeat("\u2550", 39) + "\u255d")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	cfg := config.DefaultConfig

	fmt.Print("\u4e0a\u6e38 API \u5730\u5740 (\u5982 https://api.openai.com/v1): ")
	input, _ := reader.ReadString('\n')
	cfg.Upstream.BaseURL = strings.TrimSpace(input)

	fmt.Print("API Key: ")
	input, _ = reader.ReadString('\n')
	cfg.Upstream.APIKey = strings.TrimSpace(input)

	defaultModel := cfg.Model.DefaultModel
	if defaultModel == "" {
		defaultModel = "gpt-4o"
	}
	fmt.Printf("\u6a21\u578b (\u9ed8\u8ba4 %s): ", defaultModel)
	input, _ = reader.ReadString('\n')
	if m := strings.TrimSpace(input); m != "" {
		cfg.Model.DefaultModel = m
	} else {
		cfg.Model.DefaultModel = defaultModel
	}

	fmt.Println()
	fmt.Println("\u250c" + strings.Repeat("\u2500", 35) + "\u2510")
	fmt.Printf("\u2502  \u5730\u5740: %-30s \u2502\n", cfg.Upstream.BaseURL)
	fmt.Printf("\u2502  \u5bc6\u94a5: %-30s \u2502\n", maskKey(cfg.Upstream.APIKey))
	fmt.Printf("\u2502  \u6a21\u578b: %-30s \u2502\n", cfg.Model.DefaultModel)
	fmt.Println("\u2514" + strings.Repeat("\u2500", 35) + "\u2518")
	fmt.Println()

	if err := config.Save(&cfg, "config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "\u4fdd\u5b58\u914d\u7f6e\u5931\u8d25: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("\u2713 \u914d\u7f6e\u5df2\u4fdd\u5b58\u5230 config.json")
	fmt.Println()

	if !codex.IsConfigured() {
		fmt.Print("\u662f\u5426\u914d\u7f6e\u5230 Codex CLI? (Y/n): ")
		input, _ = reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "n" {
			setupCfg := codex.SetupConfig{
				Model:   cfg.Model.DefaultModel,
				BaseURL: fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port),
				APIKey:  cfg.Upstream.APIKey,
			}
			if err := codex.Setup(setupCfg); err != nil {
				fmt.Fprintf(os.Stderr, "Codex \u914d\u7f6e\u5931\u8d25: %s\n", err)
			} else {
				fmt.Println()
				fmt.Println("\u5df2\u8bbe\u7f6e CODEX_API_KEY \u73af\u5883\u53d8\u91cf\uff0c\u8bf7\u6267\u884c:")
				fmt.Printf("  export CODEX_API_KEY=\"%s\"\n", cfg.Upstream.APIKey)
				fmt.Println("\u7136\u540e\u8fd0\u884c: codex")
			}
		}
	} else {
		fmt.Println("\u2713 \u5df2\u68c0\u6d4b\u5230 Codex CLI \u914d\u7f6e\uff0c\u8df3\u8fc7")
	}

	fmt.Println()
	fmt.Print("\u662f\u5426\u542f\u52a8\u4ee3\u7406\u670d\u52a1\u5668? (Y/n): ")
	input, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(input)) != "n" {
		fmt.Println()
		fmt.Printf("\U0001f680 \u542f\u52a8\u4ee3\u7406\u670d\u52a1\u5668\u4e8e http://127.0.0.1:%d...\n", cfg.Server.Port)
		server.Run(&cfg)
	}

	fmt.Println("\u914d\u7f6e\u5b8c\u6210, \u6309 Ctrl+C \u9000\u51fa")
	select {}
}

func maskKey(key string) string {
	if len(key) <= 6 {
		return "******"
	}
	return key[:3] + "..." + key[len(key)-3:]
}
