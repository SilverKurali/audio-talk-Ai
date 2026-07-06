package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"gitee.com/AY77-OP/audio-talk-ai/config"
	"gitee.com/AY77-OP/audio-talk-ai/engine"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	cfg     *config.Config
	history *HistoryStore
	engine  *engine.Engine
	logger  *slog.Logger
	addr    string
}

func NewServer(cfg *config.Config, eng *engine.Engine, history *HistoryStore, port int, logger *slog.Logger) *Server {
	return &Server{
		cfg:     cfg,
		history: history,
		engine:  eng,
		logger:  logger,
		addr:    fmt.Sprintf(":%d", port),
	}
}

func (s *Server) Start() {
	mux := http.NewServeMux()

	// Static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("GET /", http.FileServerFS(staticFS))

	// API routes
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	mux.HandleFunc("PUT /api/config/voice", s.handlePutVoiceConfig)
	mux.HandleFunc("GET /api/providers", s.handleGetProviders)
	mux.HandleFunc("POST /api/providers", s.handlePostProvider)
	mux.HandleFunc("PUT /api/providers/{name}", s.handlePutProvider)
	mux.HandleFunc("DELETE /api/providers/{name}", s.handleDeleteProvider)
	mux.HandleFunc("PUT /api/providers-default/{name}", s.handleSetDefault)
	mux.HandleFunc("GET /api/history", s.handleGetHistory)
	mux.HandleFunc("GET /api/status", s.handleGetStatus)

	go func() {
		s.logger.Info("WebUI started", "addr", s.addr)
		if err := http.ListenAndServe(s.addr, mux); err != nil && err != http.ErrServerClosed {
			s.logger.Error("WebUI server error", "error", err)
		}
	}()
}

func (s *Server) Addr() string {
	return s.addr
}
