package webui

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gitee.com/AY77-OP/audio-talk-ai/config"
	"gitee.com/AY77-OP/audio-talk-ai/plugins/voice"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// GET /api/config
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg)
}

// PUT /api/config
func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := readJSON(r, &cfg); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := config.Save(&cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.cfg = &cfg
	if s.engine != nil {
		s.engine.ReloadConfig(&cfg)
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// PUT /api/config/voice — only update voice settings, don't touch providers
func (s *Server) handlePutVoiceConfig(w http.ResponseWriter, r *http.Request) {
	var vc config.VoiceConfig
	if err := readJSON(r, &vc); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.cfg.Voice = vc
	if err := config.Save(s.cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if s.engine != nil {
		s.engine.ReloadConfig(s.cfg)
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/providers
func (s *Server) handleGetProviders(w http.ResponseWriter, r *http.Request) {
	providers := s.cfg.ResolveASRProviders()
	writeJSON(w, 200, providers)
}

// POST /api/providers
func (s *Server) handlePostProvider(w http.ResponseWriter, r *http.Request) {
	var p config.ASRProviderConfig
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if p.Name == "" || p.Type == "" {
		writeJSON(w, 400, map[string]string{"error": "name and type required"})
		return
	}
	if len(s.cfg.ASRs) == 0 {
		p.Default = true
	}
	s.cfg.ASRs = append(s.cfg.ASRs, p)
	if err := config.Save(s.cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if s.engine != nil {
		s.engine.ReloadConfig(s.cfg)
	}
	writeJSON(w, 201, p)
}

// PUT /api/providers/{name}
func (s *Server) handlePutProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var p config.ASRProviderConfig
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	for i, existing := range s.cfg.ASRs {
		if existing.Name == name {
			p.Name = name
			if p.Type == "" {
				p.Type = existing.Type
			}
			s.cfg.ASRs[i] = p
			if err := config.Save(s.cfg); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			if s.engine != nil {
				s.engine.ReloadConfig(s.cfg)
			}
			writeJSON(w, 200, p)
			return
		}
	}
	writeJSON(w, 404, map[string]string{"error": "provider not found"})
}

// DELETE /api/providers/{name}
func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	for i, p := range s.cfg.ASRs {
		if p.Name == name {
			s.cfg.ASRs = append(s.cfg.ASRs[:i], s.cfg.ASRs[i+1:]...)
			if len(s.cfg.ASRs) > 0 {
				hasDefault := false
				for _, pp := range s.cfg.ASRs {
					if pp.Default {
						hasDefault = true
						break
					}
				}
				if !hasDefault {
					s.cfg.ASRs[0].Default = true
				}
			}
			if err := config.Save(s.cfg); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			if s.engine != nil {
				s.engine.ReloadConfig(s.cfg)
			}
			writeJSON(w, 200, map[string]string{"status": "deleted"})
			return
		}
	}
	writeJSON(w, 404, map[string]string{"error": "provider not found"})
}

// PUT /api/providers-default/{name}
func (s *Server) handleSetDefault(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	found := false
	for i := range s.cfg.ASRs {
		if s.cfg.ASRs[i].Name == name {
			s.cfg.ASRs[i].Default = true
			found = true
		} else {
			s.cfg.ASRs[i].Default = false
		}
	}
	if !found {
		writeJSON(w, 404, map[string]string{"error": "provider not found"})
		return
	}
	if err := config.Save(s.cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if s.engine != nil {
		s.engine.ReloadConfig(s.cfg)
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/history
func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	items, total := s.history.List(offset, limit)
	sessions, chars, duration := s.history.Stats()
	writeJSON(w, 200, map[string]any{
		"items":    items,
		"total":    total,
		"sessions": sessions,
		"chars":    chars,
		"duration": duration,
	})
}

// GET /api/status
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	status := voice.TUIStatus()
	stats := voice.TUIStats()
	writeJSON(w, 200, map[string]any{
		"state":  status.State,
		"detail": status.Detail,
		"stats": map[string]any{
			"sessions": stats.Sessions,
			"chars":    stats.Chars,
			"cpm": func() float64 {
				if stats.AudioDuration > 0 {
					return float64(stats.Chars) / stats.AudioDuration.Minutes()
				}
				return 0
			}(),
		},
	})
}
