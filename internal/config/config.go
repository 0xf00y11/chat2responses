// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-20
// Package config - 配置管理 - 加载、保存和解析 chat2responses 配置文件
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type UpstreamConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type ModelConfig struct {
	DefaultModel string `json:"default_model"`
}

type ModelUpstream struct {
	BaseURL       string `json:"base_url,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	UpstreamModel string `json:"upstream_model,omitempty"` // 可选的上游真实模型名称映射
}

type GoogleOAuthConfig struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RedirectURL  string    `json:"redirect_url"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

type Config struct {
	Upstream    UpstreamConfig           `json:"upstream"`
	Server      ServerConfig             `json:"server"`
	Model       ModelConfig              `json:"model"`
	Models      map[string]ModelUpstream `json:"models,omitempty"`
	GoogleOAuth *GoogleOAuthConfig       `json:"google_oauth,omitempty"` // Google OAuth 证书和令牌缓存
	Debug       bool                     `json:"debug"`
	LoadedPath  string                   `json:"-"` // 记录本次成功加载的配置绝对路径，避免回写到错误的文件
	mu          sync.Mutex               `json:"-"` // 互斥锁，保证并发安全，避免 Token 刷新和文件保存的竞态冲突
}

var DefaultConfig = Config{
	Server: ServerConfig{Host: "0.0.0.0", Port: 57321},
}

// Load - 从指定路径或候选路径加载配置文件
func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: DefaultConfig.Server,
	}
	candidates := []string{"./config.json"}

	// 跨平台：优先使用 XDG_CONFIG_HOME（Linux/macOS），回退到 ~/.config
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "chat2responses", "config.json"))
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "chat2responses", "config.json"))
	}

	// Windows %APPDATA% 路径
	if appData := os.Getenv("APPDATA"); appData != "" {
		candidates = append(candidates, filepath.Join(appData, "chat2responses", "config.json"))
	}

	// Linux/macOS 系统级配置
	candidates = append(candidates, "/etc/chat2responses/config.json")

	if path == "" {
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
		if path == "" {
			return nil, fmt.Errorf("no config file found, create config.json or use -c")
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Upstream.BaseURL == "" {
		return nil, fmt.Errorf("upstream.base_url is required")
	}
	if cfg.Upstream.APIKey == "" {
		return nil, fmt.Errorf("upstream.api_key is required")
	}

	if absPath, err := filepath.Abs(path); err == nil {
		cfg.LoadedPath = absPath
	} else {
		cfg.LoadedPath = path
	}

	return cfg, nil
}

// ResolveModel - 解析当前请求需要的实际模型，如果请求模型为空则返回默认模型
func (c *Config) ResolveModel(requested string) string {
	if requested != "" {
		return requested
	}
	return c.Model.DefaultModel
}

// GetGoogleAccessToken - 自动获取或通过 Refresh Token 静默续签 Google Access Token，采用双检锁防止并发刷新冲突。
func (c *Config) GetGoogleAccessToken() (string, error) {
	if c.GoogleOAuth == nil {
		return "", fmt.Errorf("google_oauth is not configured")
	}

	// 1. 第一层检查：无锁快速通路
	if c.GoogleOAuth.AccessToken != "" && c.GoogleOAuth.Expiry.After(time.Now().Add(5*time.Minute)) {
		return c.GoogleOAuth.AccessToken, nil
	}

	// 2. 加锁保护并发读写和磁盘写入
	c.mu.Lock()
	defer c.mu.Unlock()

	// 3. 第二层检查（双检锁）：确保排队等锁的 goroutine 直接复用刚才已被前序请求刷新好的 Token
	if c.GoogleOAuth.AccessToken != "" && c.GoogleOAuth.Expiry.After(time.Now().Add(5*time.Minute)) {
		return c.GoogleOAuth.AccessToken, nil
	}

	if c.GoogleOAuth.RefreshToken == "" {
		return "", fmt.Errorf("google_oauth.refresh_token is empty, please run 'chat2responses login-google'")
	}

	// Token 过期，通过 Refresh Token 发起续期请求
	form := url.Values{}
	form.Set("client_id", c.GoogleOAuth.ClientID)
	form.Set("client_secret", c.GoogleOAuth.ClientSecret)
	form.Set("refresh_token", c.GoogleOAuth.RefreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gcp token refresh call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("refresh token failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	// 更新内存配置
	c.GoogleOAuth.AccessToken = tokenResp.AccessToken
	c.GoogleOAuth.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// 直接写回原加载路径，如果没有则回退至 config.json
	savePath := c.LoadedPath
	if savePath == "" {
		savePath = "config.json"
	}

	_ = Save(c, savePath) // 静默写入

	return tokenResp.AccessToken, nil
}

// Save - 保存配置到指定路径
func Save(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// GetPIDFilePath - 根据当前用户和工作区哈希获取唯一隔离的 PID 文件路径，防止多用户、多项目写冲突。
func GetPIDFilePath() string {
	wd, err := os.Getwd()
	var wdHash string
	if err == nil {
		sum := uint32(0)
		for _, c := range wd {
			sum = sum*31 + uint32(c)
		}
		wdHash = fmt.Sprintf("%08x", sum)
	} else {
		wdHash = "default"
	}

	// 跨平台：优先存放在用户缓存目录（Linux/macOS: ~/.cache, Windows: %LOCALAPPDATA%）
	if cacheDir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cacheDir, fmt.Sprintf("chat2responses_%s.pid", wdHash))
	}

	// 若获取不到缓存目录，回退到系统临时目录
	return filepath.Join(os.TempDir(), fmt.Sprintf("chat2responses_%s.pid", wdHash))
}
