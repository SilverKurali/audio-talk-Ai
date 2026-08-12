package config

import (
	"reflect"
	"testing"
)

func TestResolveASRProviders(t *testing.T) {
	t.Run("explicit ASRs returned directly", func(t *testing.T) {
		cfg := &Config{
			ASRs: []ASRProviderConfig{
				{Name: "p1", Type: "openai-whisper", Default: true},
				{Name: "p2", Type: "doubao"},
			},
		}
		got := cfg.ResolveASRProviders()
		if len(got) != 2 || got[0].Name != "p1" || got[1].Name != "p2" {
			t.Fatalf("ResolveASRProviders = %+v", got)
		}
	})

	t.Run("fallback synthesizes doubao from voice fields", func(t *testing.T) {
		cfg := &Config{
			Voice: VoiceConfig{AppKey: "ak", AccessKey: "sk", ResourceID: "rid"},
		}
		got := cfg.ResolveASRProviders()
		if len(got) != 1 {
			t.Fatalf("want 1 synthesized provider, got %d: %+v", len(got), got)
		}
		p := got[0]
		if p.Name != "doubao" || p.Type != "doubao" || !p.Default ||
			p.AppKey != "ak" || p.AccessKey != "sk" || p.ResourceID != "rid" {
			t.Fatalf("synthesized provider wrong: %+v", p)
		}
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		cfg := &Config{}
		if got := cfg.ResolveASRProviders(); got != nil {
			t.Fatalf("want nil, got %+v", got)
		}
	})
}

func TestDefaultASRProvider(t *testing.T) {
	t.Run("returns the Default provider", func(t *testing.T) {
		cfg := &Config{ASRs: []ASRProviderConfig{
			{Name: "first"}, {Name: "second", Default: true},
		}}
		if got := cfg.DefaultASRProvider(); got != "second" {
			t.Fatalf("DefaultASRProvider = %q, want %q", got, "second")
		}
	})

	t.Run("falls back to first when none marked default", func(t *testing.T) {
		cfg := &Config{ASRs: []ASRProviderConfig{{Name: "only"}}}
		if got := cfg.DefaultASRProvider(); got != "only" {
			t.Fatalf("DefaultASRProvider = %q, want %q", got, "only")
		}
	})

	t.Run("empty when no providers", func(t *testing.T) {
		cfg := &Config{}
		if got := cfg.DefaultASRProvider(); got != "" {
			t.Fatalf("DefaultASRProvider = %q, want \"\"", got)
		}
	})
}

func TestUpdateASRDefault(t *testing.T) {
	cfg := &Config{ASRs: []ASRProviderConfig{
		{Name: "a", Default: true}, {Name: "b"}, {Name: "c"},
	}}
	cfg.UpdateASRDefault("b")
	for _, p := range cfg.ASRs {
		if p.Default != (p.Name == "b") {
			t.Errorf("after UpdateASRDefault(b): %s Default=%v, want %v", p.Name, p.Default, p.Name == "b")
		}
	}
}

func TestASRProviderConfigGetSet(t *testing.T) {
	t.Run("typed field get/set", func(t *testing.T) {
		p := ASRProviderConfig{}
		p.Set("api_key", "secret123")
		if got := p.Get("api_key"); got != "secret123" {
			t.Fatalf("Get(api_key) = %q, want %q", got, "secret123")
		}
		// round-trips through the typed pointer
		if p.ApiKey != "secret123" {
			t.Fatalf("ApiKey field = %q, want %q", p.ApiKey, "secret123")
		}
	})

	t.Run("unknown key goes to Extra with lazy alloc", func(t *testing.T) {
		p := ASRProviderConfig{}
		if p.Extra != nil {
			t.Fatal("Extra should start nil")
		}
		p.Set("custom_field", "value")
		if p.Extra == nil || p.Extra["custom_field"] != "value" {
			t.Fatalf("Extra not lazily allocated / set: %+v", p.Extra)
		}
		if got := p.Get("custom_field"); got != "value" {
			t.Fatalf("Get(custom_field) = %q, want %q", got, "value")
		}
	})

	t.Run("missing key returns empty", func(t *testing.T) {
		p := ASRProviderConfig{}
		if got := p.Get("does_not_exist"); got != "" {
			t.Fatalf("Get(missing) = %q, want \"\"", got)
		}
	})
}

func TestProviderCfgMap(t *testing.T) {
	t.Run("lang maps to language output key", func(t *testing.T) {
		p := ASRProviderConfig{Lang: "zh-CN"}
		m := p.ProviderCfgMap()
		if m["language"] != "zh-CN" {
			t.Fatalf("language = %v, want zh-CN", m["language"])
		}
		if _, exists := m["lang"]; exists {
			t.Fatal("raw 'lang' key should not appear in map")
		}
	})

	t.Run("typed fields included", func(t *testing.T) {
		p := ASRProviderConfig{AppID: "A1", ApiKey: "K1", ApiSecret: "S1"}
		m := p.ProviderCfgMap()
		if m["app_id"] != "A1" || m["api_key"] != "K1" || m["api_secret"] != "S1" {
			t.Fatalf("typed fields missing: %+v", m)
		}
	})

	t.Run("extra fields merged", func(t *testing.T) {
		p := ASRProviderConfig{Extra: map[string]string{"endpoint": "https://x"}}
		m := p.ProviderCfgMap()
		if m["endpoint"] != "https://x" {
			t.Fatalf("extra not merged: %+v", m)
		}
	})
}

func TestDefaultConfigSanity(t *testing.T) {
	cfg := Default()
	// Default hotkey must parse cleanly (guards the shipped default config).
	if _, err := ParseHotkey(cfg.Voice.PushToTalk); err != nil {
		t.Fatalf("default PushToTalk %q does not parse: %v", cfg.Voice.PushToTalk, err)
	}
	if cfg.Web.Port != 8391 {
		t.Fatalf("default web port = %d, want 8391", cfg.Web.Port)
	}
	// a freshly-built Default has empty ASRs and no voice keys -> nil providers
	if providers := cfg.ResolveASRProviders(); providers != nil {
		t.Fatalf("Default() should resolve to nil providers, got %+v", providers)
	}
	// ensure no accidental field additions break this smoke test by reflection
	_ = reflect.TypeOf(Config{})
}
