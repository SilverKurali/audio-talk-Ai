package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gitee.com/AY77-OP/audio-talk-ai/hotkey"
)

type Config struct {
	Voice   VoiceConfig         `toml:"voice" json:"voice"`
	ASRs    []ASRProviderConfig `toml:"asr_providers" json:"asr_providers"`
	Debug   DebugConfig         `toml:"debug" json:"debug"`
	Overlay OverlayConfig      `toml:"overlay" json:"overlay"`
	Web     WebConfig           `toml:"web" json:"web"`
}

type ASRProviderConfig struct {
	Name       string `toml:"name" json:"name"`
	Type       string `toml:"type" json:"type"`
	Default    bool   `toml:"default,omitempty" json:"default"`
	// Doubao fields
	AppKey     string `toml:"app_key" json:"app_key"`
	AccessKey  string `toml:"access_key" json:"access_key"`
	ResourceID string `toml:"resource_id" json:"resource_id"`
	// OpenAI fields
	ApiKey  string `toml:"api_key" json:"api_key"`
	Model   string `toml:"model" json:"model"`
	BaseURL string `toml:"base_url" json:"base_url"`
	// iFlytek fields
	AppID     string `toml:"app_id" json:"app_id"`
	ApiSecret string `toml:"api_secret" json:"api_secret"`
	DWA       string `toml:"dwa" json:"dwa"`
}

// ProviderCfgMap converts the provider config to a map for the ASR factory.
func (p *ASRProviderConfig) ProviderCfgMap() map[string]interface{} {
	return map[string]interface{}{
		"app_key":     p.AppKey,
		"access_key":  p.AccessKey,
		"resource_id": p.ResourceID,
		"api_key":     p.ApiKey,
		"model":       p.Model,
		"base_url":    p.BaseURL,
		"app_id":      p.AppID,
		"api_secret":  p.ApiSecret,
		"dwa":         p.DWA,
	}
}

// ResolveASRProviders returns the configured ASR providers.
// If [[asr_providers]] is configured, returns those.
// Otherwise falls back to building a single "doubao" provider from [voice] fields.
func (c *Config) ResolveASRProviders() []ASRProviderConfig {
	if len(c.ASRs) > 0 {
		return c.ASRs
	}
	if c.Voice.AppKey != "" || c.Voice.AccessKey != "" {
		return []ASRProviderConfig{{
			Name:       "doubao",
			Type:       "doubao",
			Default:    true,
			AppKey:     c.Voice.AppKey,
			AccessKey:  c.Voice.AccessKey,
			ResourceID: c.Voice.ResourceID,
		}}
	}
	return nil
}

// DefaultASRProvider returns the name of the default ASR provider.
func (c *Config) DefaultASRProvider() string {
	providers := c.ResolveASRProviders()
	if len(providers) == 0 {
		return ""
	}
	for _, p := range providers {
		if p.Default {
			return p.Name
		}
	}
	return providers[0].Name
}

// UpdateASRDefault sets the named provider as default and clears others.
func (c *Config) UpdateASRDefault(name string) {
	for i := range c.ASRs {
		c.ASRs[i].Default = c.ASRs[i].Name == name
	}
}

type DebugConfig struct {
	Enabled bool     `toml:"enabled" json:"enabled"`
	Hotkeys []string `toml:"hotkeys" json:"hotkeys"`
}

type OverlayConfig struct {
	Enabled     bool    `toml:"enabled" json:"enabled"`
	Position    string  `toml:"position" json:"position"`
	IdleVisible bool    `toml:"idle_visible" json:"idle_visible"`
	Scale       float64 `toml:"scale" json:"scale"`
}

type WebConfig struct {
	Enabled bool `toml:"enabled" json:"enabled"`
	Port    int  `toml:"port" json:"port"`
}

type VoiceConfig struct {
	Enabled     bool     `toml:"enabled" json:"enabled"`
	Mode        string   `toml:"mode" json:"mode"`
	PushToTalk  string   `toml:"push_to_talk" json:"push_to_talk"`
	Device      string   `toml:"device" json:"device"`
	Gain        int      `toml:"gain" json:"gain"`
	StopDelayMs int      `toml:"stop_delay_ms" json:"stop_delay_ms"`
	Language    string   `toml:"language" json:"language"`
	AutoSubmit  bool     `toml:"auto_submit" json:"auto_submit"`
	AppKey      string   `toml:"app_key" json:"app_key"`
	AccessKey   string   `toml:"access_key" json:"access_key"`
	ResourceID  string   `toml:"resource_id" json:"resource_id"`
	Hotwords    []string `toml:"hotwords" json:"hotwords"`
}

func Default() *Config {
	return &Config{
		Voice: VoiceConfig{
			Enabled: true, Mode: "toggle", PushToTalk: "F9",
			Language: "zh-CN", AutoSubmit: true, ResourceID: "volc.bigasr.sauc.duration",
		},
		Overlay: OverlayConfig{
			Enabled: true, Position: "bottom-center", IdleVisible: false, Scale: 1.0,
		},
		Web: WebConfig{
			Enabled: true, Port: 8391,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = FindConfig()
	}
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func FindConfig() string {
	candidates := []string{"./config.toml"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "audio-talk-ai", "config.toml"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "audio-talk-ai", "config.toml"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func Save(cfg *Config) error {
	path := FindConfig()
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "audio-talk-ai", "config.toml")
		os.MkdirAll(filepath.Dir(path), 0755)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// ---- Hotkey parser ----

var modifierNames = map[string]hotkey.Modifier{
	"ctrl": hotkey.ModCtrl, "alt": hotkey.ModAlt, "shift": hotkey.ModShift,
	"control": hotkey.ModCtrl, "option": hotkey.ModAlt, "super": hotkey.ModSuper,
	"cmd": hotkey.ModSuper, "command": hotkey.ModSuper, "win": hotkey.ModSuper,
}

var keyNameToCode = buildKeyNameMap()

func buildKeyNameMap() map[string]hotkey.KeyCode {
	m := make(map[string]hotkey.KeyCode)
	for i := hotkey.KeyA; i <= hotkey.KeyZ; i++ {
		m[strings.ToLower(i.String())] = i
		m[i.String()] = i
	}
	for i := hotkey.Key0; i <= hotkey.Key9; i++ {
		m[i.String()] = i
	}
	for i := hotkey.KeyF1; i <= hotkey.KeyF24; i++ {
		m[strings.ToLower(i.String())] = i
		m[i.String()] = i
	}
	m["ctrl"] = hotkey.KeyCtrl
	m["control"] = hotkey.KeyCtrl
	m["alt"] = hotkey.KeyAlt
	m["option"] = hotkey.KeyAlt
	m["shift"] = hotkey.KeyShift
	m["super"] = hotkey.KeySuper
	m["cmd"] = hotkey.KeySuper
	m["command"] = hotkey.KeySuper
	m["win"] = hotkey.KeySuper
	for _, k := range []hotkey.KeyCode{
		hotkey.KeySpace, hotkey.KeyTab, hotkey.KeyEnter, hotkey.KeyEscape,
		hotkey.KeyBackspace, hotkey.KeyCapsLock,
		hotkey.KeyArrowUp, hotkey.KeyArrowDown, hotkey.KeyArrowLeft, hotkey.KeyArrowRight,
		hotkey.KeyHome, hotkey.KeyEnd, hotkey.KeyPageUp, hotkey.KeyPageDown,
		hotkey.KeyInsert, hotkey.KeyDelete,
		hotkey.KeyNum0, hotkey.KeyNum1, hotkey.KeyNum2, hotkey.KeyNum3, hotkey.KeyNum4,
		hotkey.KeyNum5, hotkey.KeyNum6, hotkey.KeyNum7, hotkey.KeyNum8, hotkey.KeyNum9,
		hotkey.KeyBacktick, hotkey.KeyMinus, hotkey.KeyEqual,
		hotkey.KeyLeftBracket, hotkey.KeyRightBracket, hotkey.KeyBackslash,
		hotkey.KeySemicolon, hotkey.KeyQuote,
		hotkey.KeyComma, hotkey.KeyPeriod, hotkey.KeySlash,
	} {
		m[strings.ToLower(k.String())] = k
		m[k.String()] = k
	}
	m["space"] = hotkey.KeySpace
	m["enter"] = hotkey.KeyEnter
	m["return"] = hotkey.KeyEnter
	m["esc"] = hotkey.KeyEscape
	m["escape"] = hotkey.KeyEscape
	m["backspace"] = hotkey.KeyBackspace
	m["tab"] = hotkey.KeyTab
	m["up"] = hotkey.KeyArrowUp
	m["down"] = hotkey.KeyArrowDown
	m["left"] = hotkey.KeyArrowLeft
	m["right"] = hotkey.KeyArrowRight
	m["home"] = hotkey.KeyHome
	m["end"] = hotkey.KeyEnd
	m["pageup"] = hotkey.KeyPageUp
	m["pagedown"] = hotkey.KeyPageDown
	m["insert"] = hotkey.KeyInsert
	m["delete"] = hotkey.KeyDelete
	m["capslock"] = hotkey.KeyCapsLock
	m["`"] = hotkey.KeyBacktick
	m["-"] = hotkey.KeyMinus
	m["="] = hotkey.KeyEqual
	m["["] = hotkey.KeyLeftBracket
	m["]"] = hotkey.KeyRightBracket
	m["\\"] = hotkey.KeyBackslash
	m[";"] = hotkey.KeySemicolon
	m["'"] = hotkey.KeyQuote
	m[","] = hotkey.KeyComma
	m["."] = hotkey.KeyPeriod
	m["/"] = hotkey.KeySlash
	return m
}

func ParseHotkey(s string) (hotkey.Combo, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return hotkey.Combo{}, fmt.Errorf("empty hotkey string")
	}
	parts := strings.Split(s, "+")
	var mods hotkey.Modifier
	var key hotkey.KeyCode
	for _, part := range parts {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		if mod, ok := modifierNames[lower]; ok {
			mods |= mod
			continue
		}
		if k, ok := keyNameToCode[lower]; ok {
			if key != hotkey.KeyNone {
				return hotkey.Combo{}, fmt.Errorf("multiple keys in %q", s)
			}
			if k.IsModifier() && key == hotkey.KeyNone {
				key = k
			} else if !k.IsModifier() {
				key = k
			}
			continue
		}
		return hotkey.Combo{}, fmt.Errorf("unknown key %q in %q", part, s)
	}
	if key == hotkey.KeyNone && mods != hotkey.ModNone {
		return hotkey.Combo{Mods: mods, Key: hotkey.KeyNone}, nil
	}
	if key != hotkey.KeyNone && mods == hotkey.ModNone && !key.IsModifier() {
		return hotkey.Combo{Mods: hotkey.ModNone, Key: key}, nil
	}
	if key.IsModifier() && mods == hotkey.ModNone {
		return hotkey.Combo{Mods: hotkey.KeyCodeToModifier(key), Key: hotkey.KeyNone}, nil
	}
	if key != hotkey.KeyNone && mods != hotkey.ModNone {
		return hotkey.Combo{Mods: mods, Key: key}, nil
	}
	return hotkey.Combo{}, fmt.Errorf("cannot parse hotkey %q", s)
}

func ParseHotkeys(strings []string) ([]hotkey.Combo, error) {
	var combos []hotkey.Combo
	for _, s := range strings {
		c, err := ParseHotkey(s)
		if err != nil {
			return nil, err
		}
		combos = append(combos, c)
	}
	return combos, nil
}
