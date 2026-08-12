package webui

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

// newTestServer builds an isolated white-box Server: every config path
// (HOME / XDG_CONFIG_HOME / cwd) points at a temp dir, config.toml is
// pre-created so config.Save (which resolves via FindConfig) writes here,
// and the engine is nil (all handlers guard `s.engine != nil`).
func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Chdir(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "config.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Server{
		cfg:     config.Default(),
		history: NewHistoryStore(tmp),
		engine:  nil,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		addr:    ":0",
	}
}

type handlerFn func(http.ResponseWriter, *http.Request)

func call(t *testing.T, h handlerFn, method, target string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, body)
	h(rr, req)
	return rr
}

// callName is like call but also sets a path value (for {name} routes).
func callName(t *testing.T, h handlerFn, method, target, name string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, body)
	req.SetPathValue("name", name)
	h(rr, req)
	return rr
}

func jsonBody(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func decode(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode response body %q: %v", rr.Body.String(), err)
	}
	return m
}

// ---- read-only handlers ----

func TestHandleGetConfig(t *testing.T) {
	s := newTestServer(t)
	rr := call(t, s.handleGetConfig, "GET", "/api/config", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d, want 200", rr.Code)
	}
	m := decode(t, rr)
	// default config has web port 8391
	web, _ := m["web"].(map[string]any)
	if web == nil || int(web["port"].(float64)) != 8391 {
		t.Errorf("config web.port = %v, want 8391", web)
	}
}

func TestHandleGetProviders(t *testing.T) {
	s := newTestServer(t)
	s.cfg.ASRs = []config.ASRProviderConfig{
		{Name: "a", Type: "openai-whisper"},
	}
	rr := call(t, s.handleGetProviders, "GET", "/api/providers", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var got []config.ASRProviderConfig
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("providers = %+v", got)
	}
}

func TestHandleGetProviderTypes(t *testing.T) {
	s := newTestServer(t)
	rr := call(t, s.handleGetProviderTypes, "GET", "/api/provider-types", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	m := decode(t, rr)
	// webui -> voice -> asr/drivers registers all 12 providers
	if len(m) != 12 {
		t.Errorf("provider types = %d entries, want 12: %+v", len(m), m)
	}
}

func TestHandleValidateHotkey(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		query string
		wantOK bool
	}{
		{"s=F9", true},
		{"s=Ctrl%2BShift%2BF9", true}, // Ctrl+Shift+F9 url-encoded
		{"s=a", false},                // plain text key rejected
		{"s=", false},                 // empty
		{"s=garbage%2Bx", false},      // unparseable
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			rr := call(t, s.handleValidateHotkey, "GET", "/api/hotkey/validate?"+tc.query, nil)
			if rr.Code != 200 {
				t.Fatalf("code = %d", rr.Code)
			}
			m := decode(t, rr)
			if m["ok"] != tc.wantOK {
				t.Errorf("ok = %v, want %v (body %v)", m["ok"], tc.wantOK, m)
			}
		})
	}
}

func TestHandleGetHistory(t *testing.T) {
	s := newTestServer(t)
	s.history.Add(HistoryEntry{Text: "hi", Provider: "doubao", Duration: 1.0})

	rr := call(t, s.handleGetHistory, "GET", "/api/history?offset=0&limit=10", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	m := decode(t, rr)
	if int(m["total"].(float64)) != 1 {
		t.Errorf("total = %v, want 1", m["total"])
	}
	if int(m["sessions"].(float64)) != 1 {
		t.Errorf("sessions = %v, want 1", m["sessions"])
	}
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items len = %d, want 1", len(items))
	}
	// default limit when limit<=0
	rr2 := call(t, s.handleGetHistory, "GET", "/api/history", nil)
	if rr2.Code != 200 {
		t.Fatalf("code = %d", rr2.Code)
	}
}

func TestHandleGetDateStatsAndExport(t *testing.T) {
	s := newTestServer(t)
	s.history.Add(HistoryEntry{Text: "x", Provider: "doubao", Duration: 2.0})

	t.Run("dates", func(t *testing.T) {
		rr := call(t, s.handleGetDateStats, "GET", "/api/history/dates", nil)
		if rr.Code != 200 {
			t.Fatalf("code = %d", rr.Code)
		}
		var stats map[string]map[string][]int
		if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
			t.Fatal(err)
		}
		if len(stats) == 0 {
			t.Error("expected at least one date")
		}
	})

	t.Run("export txt", func(t *testing.T) {
		rr := call(t, s.handleExportHistory, "GET", "/api/history/export?format=txt", nil)
		if rr.Code != 200 {
			t.Fatalf("code = %d", rr.Code)
		}
		if ct := rr.Header().Get("Content-Type"); ct == "" || ct[:len("text/plain")] != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", ct)
		}
		if rr.Body.Len() == 0 {
			t.Error("empty txt export")
		}
	})

	t.Run("export json", func(t *testing.T) {
		rr := call(t, s.handleExportHistory, "GET", "/api/history/export?format=json", nil)
		if rr.Code != 200 {
			t.Fatalf("code = %d", rr.Code)
		}
		var items []HistoryEntry
		if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Errorf("json export items = %d, want 1", len(items))
		}
	})
}

func TestHandleGetStatus(t *testing.T) {
	s := newTestServer(t)
	rr := call(t, s.handleGetStatus, "GET", "/api/status", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	m := decode(t, rr)
	if _, ok := m["state"]; !ok {
		t.Errorf("missing state in status: %v", m)
	}
	if _, ok := m["stats"]; !ok {
		t.Errorf("missing stats in status: %v", m)
	}
}

// ---- mutating handlers (isolated config dir) ----

func TestHandlePostProvider(t *testing.T) {
	t.Run("first provider becomes default", func(t *testing.T) {
		s := newTestServer(t)
		rr := call(t, s.handlePostProvider, "POST", "/api/providers",
			jsonBody(t, map[string]string{"name": "p", "type": "openai-whisper", "api_key": "k"}))
		if rr.Code != 201 {
			t.Fatalf("code = %d, want 201", rr.Code)
		}
		var got config.ASRProviderConfig
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !got.Default {
			t.Error("first provider should be auto-default")
		}
		if len(s.cfg.ASRs) != 1 {
			t.Fatalf("cfg.ASRs = %d, want 1", len(s.cfg.ASRs))
		}
	})

	t.Run("missing name/type -> 400", func(t *testing.T) {
		s := newTestServer(t)
		rr := call(t, s.handlePostProvider, "POST", "/api/providers",
			jsonBody(t, map[string]string{"name": "", "type": ""}))
		if rr.Code != 400 {
			t.Fatalf("code = %d, want 400", rr.Code)
		}
	})

	t.Run("bad json -> 400", func(t *testing.T) {
		s := newTestServer(t)
		rr := call(t, s.handlePostProvider, "POST", "/api/providers", bytes.NewReader([]byte("{bad")))
		if rr.Code != 400 {
			t.Fatalf("code = %d, want 400", rr.Code)
		}
	})
}

func TestHandlePutProvider(t *testing.T) {
	s := newTestServer(t)
	s.cfg.ASRs = []config.ASRProviderConfig{{Name: "p", Type: "openai-whisper", ApiKey: "old"}}

	t.Run("update existing", func(t *testing.T) {
		rr := callName(t, s.handlePutProvider, "PUT", "/api/providers/p", "p",
			jsonBody(t, map[string]string{"api_key": "new"}))
		if rr.Code != 200 {
			t.Fatalf("code = %d, want 200", rr.Code)
		}
		if s.cfg.ASRs[0].ApiKey != "new" {
			t.Errorf("ApiKey = %q, want new", s.cfg.ASRs[0].ApiKey)
		}
		// empty type in body keeps the existing type
		if s.cfg.ASRs[0].Type != "openai-whisper" {
			t.Errorf("Type = %q, want preserved openai-whisper", s.cfg.ASRs[0].Type)
		}
	})

	t.Run("unknown -> 404", func(t *testing.T) {
		rr := callName(t, s.handlePutProvider, "PUT", "/api/providers/zzz", "zzz",
			jsonBody(t, map[string]string{"api_key": "x"}))
		if rr.Code != 404 {
			t.Fatalf("code = %d, want 404", rr.Code)
		}
	})
}

func TestHandleDeleteProvider(t *testing.T) {
	t.Run("delete non-default, default stays", func(t *testing.T) {
		s := newTestServer(t)
		s.cfg.ASRs = []config.ASRProviderConfig{
			{Name: "a", Default: true},
			{Name: "b"},
		}
		rr := callName(t, s.handleDeleteProvider, "DELETE", "/api/providers/b", "b", nil)
		if rr.Code != 200 {
			t.Fatalf("code = %d", rr.Code)
		}
		if len(s.cfg.ASRs) != 1 || s.cfg.ASRs[0].Name != "a" || !s.cfg.ASRs[0].Default {
			t.Fatalf("unexpected remaining: %+v", s.cfg.ASRs)
		}
	})

	t.Run("delete default reassigns to first", func(t *testing.T) {
		s := newTestServer(t)
		s.cfg.ASRs = []config.ASRProviderConfig{
			{Name: "a", Default: true},
			{Name: "b"},
		}
		rr := callName(t, s.handleDeleteProvider, "DELETE", "/api/providers/a", "a", nil)
		if rr.Code != 200 {
			t.Fatalf("code = %d", rr.Code)
		}
		// after deleting the default, the first remaining must become default
		if len(s.cfg.ASRs) != 1 || !s.cfg.ASRs[0].Default {
			t.Fatalf("default not reassigned: %+v", s.cfg.ASRs)
		}
	})

	t.Run("unknown -> 404", func(t *testing.T) {
		s := newTestServer(t)
		rr := callName(t, s.handleDeleteProvider, "DELETE", "/api/providers/zzz", "zzz", nil)
		if rr.Code != 404 {
			t.Fatalf("code = %d, want 404", rr.Code)
		}
	})
}

func TestHandleSetDefault(t *testing.T) {
	s := newTestServer(t)
	s.cfg.ASRs = []config.ASRProviderConfig{
		{Name: "a", Default: true},
		{Name: "b"},
	}
	rr := callName(t, s.handleSetDefault, "PUT", "/api/providers-default/b", "b", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	if s.cfg.ASRs[0].Default || !s.cfg.ASRs[1].Default {
		t.Errorf("default toggle wrong: %+v", s.cfg.ASRs)
	}

	t.Run("unknown -> 404", func(t *testing.T) {
		rr := callName(t, s.handleSetDefault, "PUT", "/api/providers-default/zzz", "zzz", nil)
		if rr.Code != 404 {
			t.Fatalf("code = %d, want 404", rr.Code)
		}
	})
}

func TestHandlePutConfig(t *testing.T) {
	s := newTestServer(t)
	newCfg := config.Default()
	newCfg.Voice.Language = "ja-JP"
	rr := call(t, s.handlePutConfig, "PUT", "/api/config", jsonBody(t, newCfg))
	if rr.Code != 200 {
		t.Fatalf("code = %d, want 200 (body %s)", rr.Code, rr.Body.String())
	}
	if s.cfg.Voice.Language != "ja-JP" {
		t.Errorf("cfg not updated: Language = %q", s.cfg.Voice.Language)
	}
}

func TestHandlePutVoiceConfig(t *testing.T) {
	s := newTestServer(t)
	vc := config.VoiceConfig{Enabled: true, Mode: "hold", PushToTalk: "F9", Language: "en-US"}
	rr := call(t, s.handlePutVoiceConfig, "PUT", "/api/config/voice", jsonBody(t, vc))
	if rr.Code != 200 {
		t.Fatalf("code = %d, want 200", rr.Code)
	}
	if s.cfg.Voice.Mode != "hold" || s.cfg.Voice.Language != "en-US" {
		t.Errorf("voice config not applied: %+v", s.cfg.Voice)
	}
}

func TestHandleDeleteHistory(t *testing.T) {
	s := newTestServer(t)
	s.history.Add(HistoryEntry{Text: "a"})
	s.history.Add(HistoryEntry{Text: "b"})

	t.Run("by ids", func(t *testing.T) {
		rr := call(t, s.handleDeleteHistory, "DELETE", "/api/history",
			jsonBody(t, map[string]any{"ids": []int{1}}))
		if rr.Code != 200 {
			t.Fatalf("code = %d", rr.Code)
		}
		m := decode(t, rr)
		if int(m["deleted"].(float64)) != 1 {
			t.Errorf("deleted = %v, want 1", m["deleted"])
		}
	})

	t.Run("bad request when neither ids nor range", func(t *testing.T) {
		rr := call(t, s.handleDeleteHistory, "DELETE", "/api/history",
			jsonBody(t, map[string]any{}))
		if rr.Code != 400 {
			t.Fatalf("code = %d, want 400", rr.Code)
		}
	})
}
