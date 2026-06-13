// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-13
// cmd/chat2responses - chat2responses 主程序入口 - 交互式设置向导与代理服务器
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chat2responses/internal/codex"
	"chat2responses/internal/config"
	"chat2responses/internal/server"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			configPath := ""
			if len(os.Args) > 2 {
				// 支持通过 chat2responses serve /path/to/config.json 显式指定配置文件
				configPath = os.Args[2]
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "config error: %s\n", err)
				os.Exit(1)
			}
			server.Run(cfg)
			return
		case "setup":
			interactiveSetup()
			return
		case "config":
			interactiveConfig()
			return
		case "login-google":
			interactiveGoogleLogin()
			return
		case "stop":
			stopServer()
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
		fmt.Println("✓ 检测到已有配置，跳过设置向导")
		fmt.Printf("🚀 启动代理服务器于 http://127.0.0.1:%d...\n", cfg.Server.Port)
		server.Run(cfg)
		return
	}

	interactiveSetup()
}

func printUsage() {
	fmt.Println(`chat2responses - Chat Completions to Responses API proxy

USAGE:
    chat2responses                     Start proxy server (auto-detect config)
    chat2responses setup               Interactive setup wizard
    chat2responses config              Interactive configuration wizard for any custom models
    chat2responses login-google        Login with your Google account via OAuth 2.0
    chat2responses stop                Stop the running proxy server gracefully
    chat2responses serve [config_path] Start proxy server with optional config path
    chat2responses version             Show version
    chat2responses help                Show this help`)
}

func interactiveSetup() {
	fmt.Println("╔" + strings.Repeat("═", 39) + "╗")
	fmt.Println("║     chat2responses 设置向导 v0.1.0    ║")
	fmt.Println("║   Chat Completions → Responses API    ║")
	fmt.Println("╚" + strings.Repeat("═", 39) + "╝")
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
	fmt.Println("┌" + strings.Repeat("─", 35) + "┐")
	fmt.Printf("│  地址: %-30s │\n", cfg.Upstream.BaseURL)
	fmt.Printf("│  密钥: %-30s │\n", maskKey(cfg.Upstream.APIKey))
	fmt.Printf("│  模型: %-30s │\n", cfg.Model.DefaultModel)
	fmt.Println("└" + strings.Repeat("─", 35) + "┘")
	fmt.Println()

	if err := config.Save(&cfg, "config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "保存配置失败: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ 配置已保存到 config.json")
	fmt.Println()

	if !codex.IsConfigured() {
		fmt.Print("是否配置到 Codex CLI? (Y/n): ")
		input, _ = reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "n" {
			var customModels []string
			if cfg.Models != nil {
				for mID := range cfg.Models {
					customModels = append(customModels, mID)
				}
			}
			setupCfg := codex.SetupConfig{
				Model:        cfg.Model.DefaultModel,
				BaseURL:      fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port),
				APIKey:       cfg.Upstream.APIKey,
				CustomModels: customModels,
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
	} else {
		fmt.Println("✓ 已检测到 Codex CLI 配置，跳过")
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

// interactiveConfig - 提供全模型通用的交互式命令行配置协助
func interactiveConfig() {
	reader := bufio.NewReader(os.Stdin)

	cfg, err := config.Load("")
	if err != nil {
		fmt.Println("⚠ 未检测到有效的基础配置文件 config.json。")
		fmt.Print("是否为您初始化默认基础配置？(Y/n): ")
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "n" {
			fmt.Println("配置已取消，请先运行 chat2responses setup 配置基础服务。")
			return
		}
		cfgVal := config.DefaultConfig
		cfg = &cfgVal
		// 引导配置基础 upstream
		fmt.Print("请输入默认上游 API 地址 (如 https://api.openai.com/v1): ")
		input, _ = reader.ReadString('\n')
		cfg.Upstream.BaseURL = strings.TrimSpace(input)

		fmt.Print("请输入默认 API Key: ")
		input, _ = reader.ReadString('\n')
		cfg.Upstream.APIKey = strings.TrimSpace(input)

		cfg.Model.DefaultModel = "gpt-4o"
	}

	if cfg.Models == nil {
		cfg.Models = make(map[string]config.ModelUpstream)
	}

	for {
		fmt.Println()
		fmt.Println("╔" + strings.Repeat("═", 39) + "╗")
		fmt.Println("║     chat2responses 自定义模型配置助手  ║")
		fmt.Println("╚" + strings.Repeat("═", 39) + "╝")
		fmt.Println()

		fmt.Println("当前已配置的自定义模型:")
		if len(cfg.Models) == 0 {
			fmt.Println("  (无)")
		} else {
			i := 1
			for mID, mu := range cfg.Models {
				upstreamInfo := ""
				if mu.UpstreamModel != "" && mu.UpstreamModel != mID {
					upstreamInfo = fmt.Sprintf(" -> 映射为: %s", mu.UpstreamModel)
				}
				fmt.Printf("  [%d] %-20s (地址: %s%s)\n", i, mID, mu.BaseURL, upstreamInfo)
				i++
			}
		}
		fmt.Println()

		fmt.Println("请选择操作:")
		fmt.Println("  [1] 添加/修改自定义模型配置")
		fmt.Println("  [2] 删除已配置的自定义模型")
		fmt.Println("  [3] 保存并退出")
		fmt.Println()
		fmt.Print("请输入选项 [1-3] (默认 3): ")

		optStr, _ := reader.ReadString('\n')
		optStr = strings.TrimSpace(optStr)
		if optStr == "" || optStr == "3" {
			break
		}

		switch optStr {
		case "1":
			fmt.Println("\n--- 添加/修改自定义模型配置 ---")
			fmt.Print("请输入要配置的模型名称 (例如 deepseek-v4-flash): ")
			mName, _ := reader.ReadString('\n')
			mName = strings.TrimSpace(mName)
			if mName == "" {
				fmt.Println("⚠ 模型名称不能为空。")
				continue
			}

			existing, ok := cfg.Models[mName]
			mu := config.ModelUpstream{}
			if ok {
				mu = existing
			}

			defaultURL := "https://api.openai.com/v1"
			if mu.BaseURL != "" {
				defaultURL = mu.BaseURL
			} else if strings.Contains(mName, "deepseek") {
				defaultURL = "https://api.deepseek.com/v1"
			} else if strings.Contains(mName, "gemini") {
				defaultURL = "https://generativelanguage.googleapis.com/v1beta/openai"
			}

			fmt.Printf("API 地址 (当前/默认 %s): ", defaultURL)
			urlInput, _ := reader.ReadString('\n')
			urlInput = strings.TrimSpace(urlInput)
			if urlInput != "" {
				mu.BaseURL = urlInput
			} else {
				mu.BaseURL = defaultURL
			}

			keyPrompt := "API Key (输入 google_oauth 以启用谷歌三方授权): "
			if ok && mu.APIKey != "" {
				keyPrompt = fmt.Sprintf("API Key (当前 %s, 输入 google_oauth 启用谷歌授权，或留空保持不变): ", maskKey(mu.APIKey))
			}
			fmt.Print(keyPrompt)
			keyInput, _ := reader.ReadString('\n')
			keyInput = strings.TrimSpace(keyInput)
			if keyInput != "" {
				mu.APIKey = keyInput
			} else if !ok {
				fmt.Println("⚠ 新增模型必须输入 API Key。")
				continue
			}

			mappedPrompt := "上游真实模型名称 (可选，留空默认与上面一致): "
			if ok && mu.UpstreamModel != "" {
				mappedPrompt = fmt.Sprintf("上游真实模型名称 (当前 %s, 留空保持不变): ", mu.UpstreamModel)
			}
			fmt.Print(mappedPrompt)
			mapInput, _ := reader.ReadString('\n')
			mapInput = strings.TrimSpace(mapInput)
			if mapInput != "" {
				mu.UpstreamModel = mapInput
			} else if !ok {
				mu.UpstreamModel = mName
			}

			cfg.Models[mName] = mu
			fmt.Printf("✓ 模型 %s 配置成功！\n", mName)

		case "2":
			fmt.Println("\n--- 删除已配置的自定义模型 ---")
			fmt.Print("请输入要删除的模型名称: ")
			mName, _ := reader.ReadString('\n')
			mName = strings.TrimSpace(mName)
			if mName == "" {
				continue
			}
			if _, exists := cfg.Models[mName]; exists {
				delete(cfg.Models, mName)
				fmt.Printf("✓ 模型 %s 已删除。\n", mName)
			} else {
				fmt.Printf("⚠ 模型 %s 未配置，无需删除。\n", mName)
			}
		default:
			fmt.Println("⚠ 无效的选项，请重新选择。")
		}
	}

	// 保存配置
	if err := config.Save(cfg, "config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "保存配置失败: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ 配置已成功保存至 config.json")
}

// interactiveGoogleLogin - 协助用户通过谷歌 OAuth2 个人账号进行三方登录获取并续签 Token
func interactiveGoogleLogin() {
	fmt.Println("╔" + strings.Repeat("═", 39) + "╗")
	fmt.Println("║     Google 账号 OAuth 2.0 三方登录助手   ║")
	fmt.Println("╚" + strings.Repeat("═", 39) + "╝")
	fmt.Println()

	fmt.Println("提示：此登录助手用于获取和维护您的 Google Access Token 与 Refresh Token。")
	fmt.Println("请确保您已在 Google Cloud Console 中创建了 OAuth 2.0 Web 应用客户端，且配置如下：")
	fmt.Println("  1. 类型 (Application type): Web Application (Web 应用)")
	fmt.Println("  2. 重定向 URI (Authorized redirect URIs): http://localhost:8080/oauth/callback")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("请输入您的 Google Client ID: ")
	clientID, _ := reader.ReadString('\n')
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		fmt.Println("⚠ Client ID 不能为空，登录取消。")
		return
	}

	fmt.Print("请输入您的 Google Client Secret: ")
	clientSecret, _ := reader.ReadString('\n')
	clientSecret = strings.TrimSpace(clientSecret)
	if clientSecret == "" {
		fmt.Println("⚠ Client Secret 不能为空，登录取消。")
		return
	}

	// 启动本地临时授权回调捕获服务
	authCodeChan := make(chan string)
	mux := http.NewServeMux()
	srv := &http.Server{Addr: ":8080", Handler: mux}

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if code != "" {
			w.Write([]byte(`
				<html>
				<body style="font-family: system-ui, -apple-system, sans-serif; text-align: center; padding: 50px;">
					<h1 style="color: #10b981;">✓ 授权成功！</h1>
					<p style="color: #4b5563; font-size: 1.1em;">您的 Google 账号已授权通过。现在可以关闭此窗口，返回终端查看后续结果。</p>
				</body>
				</html>
			`))
			authCodeChan <- code
		} else {
			w.Write([]byte(`
				<html>
				<body style="font-family: system-ui, -apple-system, sans-serif; text-align: center; padding: 50px;">
					<h1 style="color: #ef4444;">✗ 授权失败</h1>
					<p style="color: #4b5563; font-size: 1.1em;">未能从回调中获取到授权验证 code。</p>
				</body>
				</html>
			`))
			authCodeChan <- ""
		}
	})

	go func() {
		_ = srv.ListenAndServe()
	}()

	// 拼装授权跳转连接
	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent",
		clientID,
		url.QueryEscape("http://localhost:8080/oauth/callback"),
		url.QueryEscape("https://www.googleapis.com/auth/cloud-platform"), // Scope 能无阻访问 Vertex AI
	)

	fmt.Println()
	fmt.Println("👉 请在浏览器中打开并同意以下 Google OAuth 链接：")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("正在本地端口 :8080 监听回调验证信号，完成授权后将自动进行下一步...")

	// 堵塞等待回调通知
	code := <-authCodeChan
	_ = srv.Close() // 授权完成，立刻关闭临时端口

	if code == "" {
		fmt.Println("⚠ 授权 code 抓取为空，登录中止。")
		return
	}

	fmt.Println("\n正在向 Google API 发送验证凭据以换取 Access Token 与 Refresh Token...")

	// 换取 Token
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", "http://localhost:8080/oauth/callback")
	form.Set("grant_type", "authorization_code")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		fmt.Printf("✗ 换取 Token 通信失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("✗ 换票失败 Google 返回 [%d]: %s\n", resp.StatusCode, string(body))
		return
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		fmt.Printf("✗ 解析 Token 响应数据失败: %v\n", err)
		return
	}

	// 加载并重新回写至配置文件
	cfg, err := config.Load("")
	if err != nil {
		fmt.Println("⚠ 未能成功加载基础配置文件以记录，为其新建配置结构...")
		cfgVal := config.DefaultConfig
		cfg = &cfgVal
	}

	cfg.GoogleOAuth = &config.GoogleOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  "http://localhost:8080/oauth/callback",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	if err := config.Save(cfg, "config.json"); err != nil {
		fmt.Printf("✗ 登录成功但保存 config.json 失败: %v\n", err)
		return
	}

	fmt.Println()
	fmt.Println("╔" + strings.Repeat("═", 39) + "╗")
	fmt.Println("║       Google 账号 OAuth2 授权登录成功！   ║")
	fmt.Println("╚" + strings.Repeat("═", 39) + "╝")
	fmt.Println()
	fmt.Printf("  Access Token:  %s\n", maskKey(tokenResp.AccessToken))
	fmt.Printf("  Refresh Token: %s\n", maskKey(tokenResp.RefreshToken))
	fmt.Printf("  过期缓冲时间:   %s (%d 秒)\n", time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second).Format("2006-01-02 15:04:05"), tokenResp.ExpiresIn)
	fmt.Println()
	fmt.Println("现在，您只需要在 'config.json' 里指定模型的 api_key 为 \"google_oauth\"，")
	fmt.Println("代理即可在后台为您静默换票，源源不断地提供 OAuth2 认证支持了！")
}

// stopServer - 读取 PID 文件并向该进程发送信号，优雅一键关闭服务
func stopServer() {
	pidFile := filepath.Join(os.TempDir(), "chat2responses.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("⚠ 未检测到正在运行的 chat2responses 代理服务 (PID 文件不存在)。")
		return
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		fmt.Println("⚠ PID 文件数据损坏，正在清理废弃 PID 文件...")
		_ = os.Remove(pidFile)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("⚠ 无法定位进程 PID %d，正在清理废弃 PID 文件...\n", pid)
		_ = os.Remove(pidFile)
		return
	}

	// 优雅发送关闭信号 (Interrupt)
	fmt.Printf("正在优雅发送关闭信号至 chat2responses 代理服务 (PID: %d)...\n", pid)
	if err := proc.Signal(os.Interrupt); err != nil {
		// 信号发送失败，尝试强制结束
		fmt.Println("⚠ 发送优雅关闭信号失败，正在尝试强制结束进程...")
		_ = proc.Kill()
		_ = os.Remove(pidFile)
		fmt.Println("✓ 服务已被强制结束。")
		return
	}

	// 轮询检查 PID 文件是否被 server 正常释放（等待 3 秒）
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(pidFile); os.IsNotExist(err) {
			fmt.Println("✓ 服务已安全停止并成功释放端口。")
			return
		}
	}

	// 超时强制结束
	fmt.Println("⚠ 服务未响应优雅关闭信号，正在进行强制性终止...")
	_ = proc.Kill()
	_ = os.Remove(pidFile)
	fmt.Println("✓ 服务已被强制终止。")
}

func maskKey(key string) string {
	if len(key) <= 6 {
		return "******"
	}
	return key[:3] + "..." + key[len(key)-3:]
}
