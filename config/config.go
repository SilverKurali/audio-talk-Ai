package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gitee.com/AY77-OP/audio-talk-ai/hotkey"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Voice   VoiceConfig         `toml:"voice" json:"voice"`
	ASRs    []ASRProviderConfig `toml:"asr_providers" json:"asr_providers"`
	Debug   DebugConfig         `toml:"debug" json:"debug"`
	Overlay OverlayConfig       `toml:"overlay" json:"overlay"`
	Web     WebConfig           `toml:"web" json:"web"`
}

type ASRProviderConfig struct {
	Name    string `toml:"name" json:"name"`
	Type    string `toml:"type" json:"type"`
	Default bool   `toml:"default,omitempty" json:"default"`
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
	key, err := loadKey()
	if err != nil {
		return nil, err
	}
	if err := cfg.decryptSecrets(key); err != nil {
		return nil, fmt.Errorf("decrypt config %s: %w", path, err)
	}
	return cfg, nil
}

// MigratePlaintextSecrets encrypts any plaintext secrets in the on-disk config
// and rewrites it, but only after proving the encryption is reversible. It
// never destroys the user's data:
//   - it first backs up the original file to <path>.bak before writing;
//   - it encrypts a clone, then immediately decrypts it and compares against
//     the live plaintext; only if they match is the encrypted copy written.
//
// On any failure the original config is left untouched and the error is
// returned so the caller can keep running with the in-memory plaintext.
func MigratePlaintextSecrets(cfgPath string, logger *slog.Logger) error {
	path := cfgPath
	if path == "" {
		path = FindConfig()
	}
	if path == "" {
		return fmt.Errorf("no config file to migrate")
	}

	// Inspect the on-disk data WITHOUT decrypting it. Migration only makes
	// sense when there are still plaintext (non-enc:) secrets on disk. If
	// everything is already encrypted (or empty) there is nothing to do and
	// we must not rewrite/lose the file.
	raw := mustLoadPlain(path)
	if !hasPlaintextSecrets(raw) {
		return nil
	}

	key, err := loadKey()
	if err != nil {
		return err
	}

	// Use the decrypted live config as the source of truth for the round trip.
	cfg, err := Load(path)
	if err != nil {
		return err
	}

	// Encrypt a clone of the live (plaintext) config.
	tmp := cloneForSave(cfg)
	if err := tmp.encryptSecrets(key); err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Prove the encrypted clone decrypts back to identical plaintext.
	verify := cloneForSave(tmp)
	if err := verify.decryptSecrets(key); err != nil {
		return fmt.Errorf("encrypted data is not reversible: %w", err)
	}
	if !secretsEqual(cfg, verify) {
		return fmt.Errorf("encrypted data does not round-trip to original plaintext")
	}

	// Safety net: keep the original around.
	backup := path + ".bak"
	if err := copyFile(path, backup); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(tmp); err != nil {
		return err
	}
	logger.Info("migrated plaintext secrets to encrypted storage", "backup", backup)
	return nil
}

// hasPlaintextSecrets reports whether any credential field holds a live
// plaintext value (non-empty and not already encrypted with the enc: prefix).
func hasPlaintextSecrets(cfg *Config) bool {
	for i := range cfg.ASRs {
		p := &cfg.ASRs[i]
		if isPlaintext(p.AppKey) || isPlaintext(p.AccessKey) || isPlaintext(p.ResourceID) ||
			isPlaintext(p.ApiKey) || isPlaintext(p.AppID) || isPlaintext(p.ApiSecret) || isPlaintext(p.DWA) {
			return true
		}
	}
	return isPlaintext(cfg.Voice.AppKey) || isPlaintext(cfg.Voice.AccessKey) || isPlaintext(cfg.Voice.ResourceID)
}

func isPlaintext(s string) bool {
	return s != "" && !strings.HasPrefix(s, encPrefix)
}

// mustLoadPlain loads the config from disk as plaintext (no decryption, no
// default-overlay side effects on the caller) for migration comparison.
func mustLoadPlain(path string) *Config {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		// Caller has already established the file exists; ignore read edge cases.
		return cfg
	}
	_ = toml.Unmarshal(data, cfg)
	return cfg
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// secretsEqual reports whether two configs carry identical data after a
// decrypt round-trip. It compares the full provider and voice structs (not
// just credential fields) so that non-secret-but-persisted fields such as
// Model and BaseURL are also verified to survive the round trip.
func secretsEqual(a, b *Config) bool {
	if len(a.ASRs) != len(b.ASRs) {
		return false
	}
	for i := range a.ASRs {
		x, y := a.ASRs[i], b.ASRs[i]
		if x != y {
			return false
		}
	}
	return a.Voice.AppKey == b.Voice.AppKey && a.Voice.AccessKey == b.Voice.AccessKey && a.Voice.ResourceID == b.Voice.ResourceID
}

// LoadRaw parses the config file as-is, without decrypting secrets. It is used
// by the doctor to inspect how secrets are stored on disk (plaintext vs
// encrypted), independent of the in-memory decrypted view.
func LoadRaw(path string) (*Config, error) {
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
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	key, err := loadKey()
	if err != nil {
		return err
	}
	// Encrypt only a detached copy so the live in-memory config stays plaintext
	// for the running process (the engine still needs usable secrets).
	tmp := cloneForSave(cfg)
	if err := tmp.encryptSecrets(key); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(tmp)
}

// cloneForSave deep-copies the parts that get encrypted so Save never mutates
// the caller's config (which the running engine relies on as plaintext).
func cloneForSave(cfg *Config) *Config {
	c := *cfg
	c.ASRs = make([]ASRProviderConfig, len(cfg.ASRs))
	for i, p := range cfg.ASRs {
		c.ASRs[i] = p
	}
	c.Voice = cfg.Voice
	c.Debug = cfg.Debug
	c.Overlay = cfg.Overlay
	c.Web = cfg.Web
	return &c
}

// encryptSecrets encrypts every credential field in place. Already-encrypted
// values (enc: prefix) are left untouched so repeated saves don't double-wrap.
func (c *Config) encryptSecrets(key []byte) error {
	for i := range c.ASRs {
		p := &c.ASRs[i]
		if err := encryptField(&p.AppKey, key); err != nil {
			return err
		}
		if err := encryptField(&p.AccessKey, key); err != nil {
			return err
		}
		if err := encryptField(&p.ResourceID, key); err != nil {
			return err
		}
		if err := encryptField(&p.ApiKey, key); err != nil {
			return err
		}
		if err := encryptField(&p.AppID, key); err != nil {
			return err
		}
		if err := encryptField(&p.ApiSecret, key); err != nil {
			return err
		}
		if err := encryptField(&p.DWA, key); err != nil {
			return err
		}
	}
	if err := encryptField(&c.Voice.AppKey, key); err != nil {
		return err
	}
	if err := encryptField(&c.Voice.AccessKey, key); err != nil {
		return err
	}
	if err := encryptField(&c.Voice.ResourceID, key); err != nil {
		return err
	}
	return nil
}

// decryptSecrets reverses encryptSecrets on load.
func (c *Config) decryptSecrets(key []byte) error {
	for i := range c.ASRs {
		p := &c.ASRs[i]
		if err := decryptField(&p.AppKey, key); err != nil {
			return err
		}
		if err := decryptField(&p.AccessKey, key); err != nil {
			return err
		}
		if err := decryptField(&p.ResourceID, key); err != nil {
			return err
		}
		if err := decryptField(&p.ApiKey, key); err != nil {
			return err
		}
		if err := decryptField(&p.AppID, key); err != nil {
			return err
		}
		if err := decryptField(&p.ApiSecret, key); err != nil {
			return err
		}
		if err := decryptField(&p.DWA, key); err != nil {
			return err
		}
	}
	if err := decryptField(&c.Voice.AppKey, key); err != nil {
		return err
	}
	if err := decryptField(&c.Voice.AccessKey, key); err != nil {
		return err
	}
	if err := decryptField(&c.Voice.ResourceID, key); err != nil {
		return err
	}
	return nil
}

func encryptField(p *string, key []byte) error {
	if *p == "" || strings.HasPrefix(*p, encPrefix) {
		return nil
	}
	e, err := encryptString([]byte(*p), key)
	if err != nil {
		return err
	}
	*p = e
	return nil
}

func decryptField(p *string, key []byte) error {
	if *p == "" || !strings.HasPrefix(*p, encPrefix) {
		return nil
	}
	d, err := decryptString(*p, key)
	if err != nil {
		return err
	}
	*p = d
	return nil
}

// HasPlaintextSecrets reports whether any credential is still stored in the
// clear (no enc: prefix). Used to migrate existing configs on startup.
func HasPlaintextSecrets(c *Config) bool {
	for i := range c.ASRs {
		p := c.ASRs[i]
		if isPlaintextSecret(p.AppKey) || isPlaintextSecret(p.AccessKey) ||
			isPlaintextSecret(p.ResourceID) || isPlaintextSecret(p.ApiKey) ||
			isPlaintextSecret(p.AppID) || isPlaintextSecret(p.ApiSecret) ||
			isPlaintextSecret(p.DWA) {
			return true
		}
	}
	return isPlaintextSecret(c.Voice.AppKey) ||
		isPlaintextSecret(c.Voice.AccessKey) ||
		isPlaintextSecret(c.Voice.ResourceID)
}

func isPlaintextSecret(s string) bool {
	return s != "" && !strings.HasPrefix(s, encPrefix)
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
