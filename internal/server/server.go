package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"chat2responses/internal/config"
	"chat2responses/internal/model"
	"chat2responses/internal/proxy"
	"chat2responses/internal/session"
)

type Server struct {
	cfg     *config.Config
	client  *proxy.UpstreamClient
	session *session.Store
	mux     *http.ServeMux
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:     cfg,
		client:  proxy.NewUpstreamClient(cfg.Upstream.BaseURL, cfg.Upstream.APIKey, cfg.Model.DefaultModel),
		session: session.NewStore(),
		mux:     http.NewServeMux(),
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

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	data, err := s.client.ListModels()
	if err != nil {
		slog.Error("list models", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
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

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Request-Id", respID)

		upstreamBody, err := s.client.ChatCompletionStream(chatReq)
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
	chatResp, err := s.client.ChatCompletion(chatReq)
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
