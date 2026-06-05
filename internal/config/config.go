// Package config - 配置管理 - 加载、保存和解析 chat2responses 配置文件
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

type Config struct {
	Upstream UpstreamConfig `json:"upstream"`
	Server   ServerConfig   `json:"server"`
	Model    ModelConfig    `json:"model"`
	Debug    bool           `json:"debug"`
}

var DefaultConfig = Config{
	Server: ServerConfig{Host: "0.0.0.0", Port: 8000},
}

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

func (c *Config) ResolveModel(requested string) string {
	if requested != "" {
		return requested
	}
	return c.Model.DefaultModel
}

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
