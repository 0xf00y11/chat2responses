// Package server - chat2responses HTTP 服务端 - 提供 Responses API 代理端点
// Copyright (c) 2026 fooyii.
// Created: 2026-05-22

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"chat2responses/internal/config"
	"chat2responses/internal/model"
	"chat2responses/internal/proxy"
)

type Server struct {
	cfg    *config.Config
	client *proxy.UpstreamClient
	mux    *http.ServeMux
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:    cfg,
		client: proxy.NewUpstreamClient(cfg.Upstream.BaseURL, cfg.Upstream.APIKey, cfg.Model.DefaultModel),
		mux:    http.NewServeMux(),
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
	chatReq := proxy.BuildChatRequest(&req)

	slog.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"model", chatReq.Model,
		"stream", chatReq.Stream,
		"messages", len(chatReq.Messages),
		"tools", len(chatReq.Tools),
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

		converter := proxy.NewStreamConverter(chatReq.Model, respID)
		if err := converter.Convert(upstreamBody, w); err != nil {
			slog.Error("stream convert", "error", err)
		}
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

