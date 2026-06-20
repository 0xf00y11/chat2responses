// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-20
// Package proxy - 上游 API 客户端 - 转发 Chat Completions 请求并处理流式/非流式响应
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"chat2responses/internal/model"
)

type UpstreamClient struct {
	baseURL       string
	apiKey        string
	defModel      string
	http          *http.Client
	tokenProvider func() (string, error) // 动态令牌提供者，主要用于 OAuth 续期
}

func NewUpstreamClient(baseURL, apiKey, defModel string) *UpstreamClient {
	return &UpstreamClient{
		baseURL:  baseURL,
		apiKey:   apiKey,
		defModel: defModel,
		http: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// SetTokenProvider - 注入动态 Token 提供器函数
func (c *UpstreamClient) SetTokenProvider(fn func() (string, error)) {
	c.tokenProvider = fn
}

func (c *UpstreamClient) ChatCompletion(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
	if req.Model == "" {
		req.Model = c.defModel
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 动态获取最新的 Bearer Token (若已配置 Token 刷新器)
	token := c.apiKey
	if c.tokenProvider != nil {
		if t, err := c.tokenProvider(); err == nil {
			token = t
		}
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp model.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &chatResp, nil
}

func (c *UpstreamClient) ChatCompletionStream(ctx context.Context, req *model.ChatRequest) (io.ReadCloser, error) {
	if req.Model == "" {
		req.Model = c.defModel
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 动态获取最新的 Bearer Token (若已配置 Token 刷新器)
	token := c.apiKey
	if c.tokenProvider != nil {
		if t, err := c.tokenProvider(); err == nil {
			token = t
		}
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (c *UpstreamClient) ListModels() ([]byte, error) {
	httpReq, err := http.NewRequest("GET", c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	token := c.apiKey
	if c.tokenProvider != nil {
		if t, err := c.tokenProvider(); err == nil {
			token = t
		}
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
