// Author: fooyii, Email: fooyii@icloud.com, Date: 2026-06-13
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"chat2responses/internal/codex"
	"chat2responses/internal/config"
	"chat2responses/internal/model"
	"chat2responses/internal/proxy"
	"chat2responses/internal/session"
)

type Server struct {
	cfg       *config.Config
	client    *proxy.UpstreamClient
	clients   map[string]*proxy.UpstreamClient // 通用多模型客户端缓存，格式：{modelName: client}
	clientsMu sync.RWMutex                     // 读写锁保护 map 并发读写
	session   *session.Store
	mux       *http.ServeMux
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:     cfg,
		client:  proxy.NewUpstreamClient(cfg.Upstream.BaseURL, cfg.Upstream.APIKey, cfg.Model.DefaultModel),
		clients: make(map[string]*proxy.UpstreamClient),
		session: session.NewStore(),
		mux:     http.NewServeMux(),
	}

	// 若全局上游使用 Google OAuth
	if cfg.Upstream.APIKey == "google_oauth" {
		s.client.SetTokenProvider(cfg.GetGoogleAccessToken)
	}

	// 预先初始化配置的所有模型客户端
	if cfg.Models != nil {
		for mID, mu := range cfg.Models {
			if mu.BaseURL != "" && mu.APIKey != "" {
				actualModel := mID
				if mu.UpstreamModel != "" {
					actualModel = mu.UpstreamModel
				}
				client := proxy.NewUpstreamClient(mu.BaseURL, mu.APIKey, actualModel)
				if mu.APIKey == "google_oauth" {
					client.SetTokenProvider(cfg.GetGoogleAccessToken)
				}
				s.clients[mID] = client
			}
		}
	}

	s.mux.HandleFunc("POST /v1/responses", s.handleResponses)
	s.mux.HandleFunc("GET /v1/models", s.handleModels)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "0.1.0",
	})
}

// getClientForModel - 根据请求的模型名称路由至正确的上游客户端实例，并返回该客户端对应要请求的真实模型名
func (s *Server) getClientForModel(modelName string) (*proxy.UpstreamClient, string) {
	if modelName == "" {
		modelName = s.cfg.ResolveModel("")
	}

	actualModel := modelName

	// 检查自定义模型映射配置
	if s.cfg.Models != nil {
		if mu, exists := s.cfg.Models[modelName]; exists {
			if mu.UpstreamModel != "" {
				actualModel = mu.UpstreamModel
			}
			// 如果该模型配置了专属的 BaseURL 和 APIKey，则需要专属 client
			if mu.BaseURL != "" && mu.APIKey != "" {
				s.clientsMu.RLock()
				c, ok := s.clients[modelName]
				s.clientsMu.RUnlock()
				if ok && c != nil {
					return c, actualModel
				}

				// 懒加载并双检锁
				s.clientsMu.Lock()
				defer s.clientsMu.Unlock()
				if c, ok = s.clients[modelName]; ok && c != nil {
					return c, actualModel
				}

				client := proxy.NewUpstreamClient(mu.BaseURL, mu.APIKey, actualModel)
				if mu.APIKey == "google_oauth" {
					client.SetTokenProvider(s.cfg.GetGoogleAccessToken)
				}
				s.clients[modelName] = client
				return client, actualModel
			}
		}
	}

	// 否则直接使用默认 client，并返回可能由 upstream_model 指定的重命名后的真实模型名
	return s.client, actualModel
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	var models []map[string]interface{}
	if data, err := s.client.ListModels(); err == nil {
		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		if json.Unmarshal(data, &resp) == nil && len(resp.Data) > 0 {
			models = resp.Data
		}
	}

	// 保证我们所有在 Models 配置中自定义的模型也列在其中
	modelIDs := make(map[string]bool)
	for _, m := range models {
		if id, ok := m["id"].(string); ok {
			modelIDs[id] = true
		}
	}

	if s.cfg.Models != nil {
		for mID := range s.cfg.Models {
			if !modelIDs[mID] {
				models = append(models, map[string]interface{}{
					"id":       mID,
					"object":   "model",
					"created":  time.Now().Unix(),
					"owned_by": "chat2responses",
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(models) == 0 {
		models = []map[string]interface{}{
			{
				"id":       s.cfg.Model.DefaultModel,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "chat2responses",
			},
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("read body", "error", err)
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	// Downgraded to Debug to protect sensitive credentials/prompt leakage (Finding 5)
	slog.Debug("raw request body", "body", string(body))

	var req model.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Error("parse request", "error", err)
		http.Error(w, fmt.Sprintf("parse error: %s", err), http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		req.Model = s.cfg.ResolveModel("")
	}

	respID := model.MakeID()

	// Build messages from this request
	newMessages := proxy.InputToMessages(&req, s.session.FindThoughtSignature)

	// Look up previous session to reconstruct the full conversation
	var fullMessages []model.ChatMessage
	if req.PreviousResponseID != "" {
		if history := s.session.Get(req.PreviousResponseID); history != nil {
			fullMessages = append(fullMessages, history...)
		} else {
			slog.Warn("previous_response_id not found, starting fresh",
				"prev_id", req.PreviousResponseID,
			)
		}
	}

	// If this is the first request in a session, include instructions as system message
	isNewSession := len(fullMessages) == 0
	if isNewSession && req.Instructions != "" {
		fullMessages = append(fullMessages, model.ChatMessage{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	fullMessages = append(fullMessages, newMessages...)

	chatReq := &model.ChatRequest{
		Model:             req.Model,
		Messages:          fullMessages,
		Stream:            req.Stream,
		MaxTokens:         req.MaxOutputTokens,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		ParallelToolCalls: req.ParallelToolCalls,
		ToolChoice:        req.ToolChoice,
	}

	// Build tools
	for _, t := range req.Tools {
		name := t.Name
		desc := t.Description
		params := t.Parameters
		if t.Function != nil {
			if name == "" {
				name = t.Function.Name
			}
			if desc == "" {
				desc = t.Function.Description
			}
			if params == nil {
				params = t.Function.Parameters
			}
		}
		if name == "" {
			continue
		}
		chatReq.Tools = append(chatReq.Tools, model.ChatTool{
			Type: "function",
			Function: &model.ChatToolFunction{
				Name:        name,
				Description: desc,
				Parameters:  params,
			},
		})
	}

	if chatReq.ToolChoice != nil {
		if tc, ok := chatReq.ToolChoice.(map[string]interface{}); ok {
			if tc["type"] == "function" && tc["function"] == nil {
				if name, ok := tc["name"].(string); ok && name != "" {
					delete(tc, "name")
					tc["function"] = map[string]interface{}{"name": name}
				}
			}
		}
	}

	start := time.Now()

	toolNames := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		name := t.Name
		if name == "" && t.Function != nil {
			name = t.Function.Name
		}
		if name != "" {
			toolNames = append(toolNames, name)
		}
	}

	slog.Info("request",
		"req_id", respID,
		"prev_id", req.PreviousResponseID,
		"method", r.Method,
		"path", r.URL.Path,
		"model", chatReq.Model,
		"stream", chatReq.Stream,
		"messages", len(chatReq.Messages),
		"instructions", len(req.Instructions) > 0,
		"max_tokens", req.MaxOutputTokens,
		"temperature", req.Temperature,
		"tools", len(req.Tools),
		"tool_names", toolNames,
		"tool_choice", req.ToolChoice,
		"body_bytes", len(body),
	)

	// 获取路由对应的具体 client 实例，以及其真正的上游模型名称
	client, actualModel := s.getClientForModel(chatReq.Model)
	chatReq.Model = actualModel // 替换为真正上游请求的模型名（别名映射）

	if chatReq.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Request-Id", respID)

		upstreamBody, err := client.ChatCompletionStream(chatReq)
		if err != nil {
			slog.Error("upstream stream", "error", err)
			http.Error(w, fmt.Sprintf("upstream error: %s", err), http.StatusBadGateway)
			return
		}

		// For streaming, we collect the assistant response to store in session
		result := proxy.NewStreamConverter(chatReq.Model, respID)
		if err := result.Convert(upstreamBody, w); err != nil {
			slog.Error("stream convert", "error", err)
		}

		// Store session: existing history + new user input + assistant response
		assistantMsg := model.ChatMessage{
			Role:             "assistant",
			Content:          result.CollectedText(),
			ReasoningContent: result.CollectedReasoning(), // Preserve thinking chain (Finding 1)
		}
		fullWithResponse := append(fullMessages, assistantMsg)
		if result.CollectedToolCalls() != nil {
			fullWithResponse[len(fullWithResponse)-1].ToolCalls = result.CollectedToolCalls()
		}
		if result.CollectedText() != "" || result.CollectedReasoning() != "" || len(result.CollectedToolCalls()) > 0 {
			s.session.Set(respID, fullWithResponse)
		}

		slog.Info("completed",
			"req_id", respID,
			"model", chatReq.Model,
			"duration", time.Since(start).String(),
		)
		return
	}

	// Non-streaming
	chatResp, err := client.ChatCompletion(chatReq)
	if err != nil {
		slog.Error("upstream", "error", err)
		http.Error(w, fmt.Sprintf("upstream error: %s", err), http.StatusBadGateway)
		return
	}

	resp := proxy.ChatToResponses(chatResp, chatReq.Model, respID)

	// Store session: existing history + new user input + assistant response
	if len(chatResp.Choices) > 0 {
		assistantMsg := chatResp.Choices[0].Message
		fullWithResponse := append(fullMessages, assistantMsg)
		s.session.Set(respID, fullWithResponse)
	}

	usage := ""
	if resp.Usage != nil {
		usage = fmt.Sprintf("in=%d out=%d total=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.TotalTokens)
	}

	slog.Info("completed",
		"req_id", respID,
		"model", chatReq.Model,
		"duration", time.Since(start).String(),
		"usage", usage,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func Run(cfg *config.Config) {
	// 写入当前进程的 PID 文件，方便后续一键优雅关闭
	pidFile := filepath.Join(os.TempDir(), "chat2responses.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		slog.Warn("Failed to write PID file", "error", err)
	}
	defer os.Remove(pidFile)

	// Automatically check and align Codex CLI's config before starting the server
	if err := codex.AutoCheckAndFix(cfg.Server.Port, cfg.Model.DefaultModel, cfg.Upstream.APIKey); err != nil {
		slog.Warn("Failed to automatically verify or correct Codex CLI configuration", "error", err)
	}

	s := New(cfg)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	slog.Info("starting chat2responses",
		"addr", addr,
		"upstream", cfg.Upstream.BaseURL,
		"model", cfg.Model.DefaultModel,
	)
	if err := http.ListenAndServe(addr, s); err != nil {
		slog.Error("server", "error", err)
		os.Exit(1)
	}
}
