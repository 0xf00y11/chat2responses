// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-13
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
}

var DefaultConfig = Config{
	Server: ServerConfig{Host: "0.0.0.0", Port: 8000},
}

// Load - 从指定路径或候选路径加载配置文件
func Load(path string) (*Config, error) {
	cfg := DefaultConfig
	candidates := []string{"./config.json"}
	if home := os.Getenv("XDG_CONFIG_HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, "chat2responses", "config.json"))
	} else if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "chat2responses", "config.json"))
	}
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

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Upstream.BaseURL == "" {
		return nil, fmt.Errorf("upstream.base_url is required")
	}
	if cfg.Upstream.APIKey == "" {
		return nil, fmt.Errorf("upstream.api_key is required")
	}
	return &cfg, nil
}

// ResolveModel - 解析当前请求需要的实际模型，如果请求模型为空则返回默认模型
func (c *Config) ResolveModel(requested string) string {
	if requested != "" {
		return requested
	}
	return c.Model.DefaultModel
}

// GetGoogleAccessToken - 自动获取或通过 Refresh Token 静默续签 Google Access Token
func (c *Config) GetGoogleAccessToken() (string, error) {
	if c.GoogleOAuth == nil {
		return "", fmt.Errorf("google_oauth is not configured")
	}

	// 如果 Access Token 依然有效（留出 5 分钟的安全缓冲），直接返回它
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

	// 找出目前的 config.json 实际存放路径并进行回写持久化
	candidates := []string{"./config.json"}
	if home := os.Getenv("XDG_CONFIG_HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, "chat2responses", "config.json"))
	} else if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "chat2responses", "config.json"))
	}

	savePath := "config.json"
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			savePath = cand
			break
		}
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
