package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfigDir redirects every config/key path into a temp dir:
//   - HOME         -> temp (controls os.UserHomeDir fallbacks in Save/FindConfig)
//   - XDG_CONFIG_HOME -> temp (controls crypto keyDir())
//   - cwd          -> temp (so FindConfig's "./config.toml" candidate lives here)
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Chdir(tmp)
	return tmp
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp) // key dir must exist for any decrypt attempt
	cfg, err := Load(filepath.Join(tmp, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load(missing) error: %v", err)
	}
	def := Default()
	if cfg.Voice.PushToTalk != def.Voice.PushToTalk || cfg.Web.Port != def.Web.Port {
		t.Fatalf("Load(missing) did not return defaults: %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := isolateConfigDir(t)

	// Pre-create config.toml so FindConfig()'s "./config.toml" candidate resolves
	// to the temp dir (Save uses FindConfig to pick its write target).
	configPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	original := Default()
	original.Voice.Language = "en-US"
	original.ASRs = []ASRProviderConfig{
		{Name: "whisper", Type: "openai-whisper", Default: true, ApiKey: "sk-test-secret-value"},
		{Name: "xf", Type: "xfyun-spark", AppID: "app-123"},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// On disk, the secret must be encrypted (not plaintext).
	onDisk, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if strings.Contains(string(onDisk), "sk-test-secret-value") {
		t.Errorf("plaintext secret leaked to disk:\n%s", onDisk)
	}
	if !strings.Contains(string(onDisk), "enc:") {
		t.Errorf("expected an enc: prefix on disk, got:\n%s", onDisk)
	}

	// Load must decrypt and restore the original values.
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Voice.Language != "en-US" {
		t.Errorf("Voice.Language = %q, want en-US", loaded.Voice.Language)
	}
	if len(loaded.ASRs) != 2 {
		t.Fatalf("loaded %d providers, want 2", len(loaded.ASRs))
	}
	// find whisper and assert decrypted secret
	var whisper *ASRProviderConfig
	for i := range loaded.ASRs {
		if loaded.ASRs[i].Name == "whisper" {
			whisper = &loaded.ASRs[i]
		}
	}
	if whisper == nil {
		t.Fatal("whisper provider missing after round-trip")
	}
	if whisper.ApiKey != "sk-test-secret-value" {
		t.Errorf("ApiKey decrypted = %q, want sk-test-secret-value", whisper.ApiKey)
	}
	if !whisper.Default {
		t.Error("whisper should still be the default after round-trip")
	}
}

func TestLoadRawDoesNotDecrypt(t *testing.T) {
	tmp := isolateConfigDir(t)
	configPath := filepath.Join(tmp, "config.toml")
	// Pre-create so Save (which uses FindConfig) writes here.
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	cfg.ASRs = []ASRProviderConfig{{Name: "p", Type: "openai-whisper", ApiKey: "sk-raw-test"}}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := LoadRaw(configPath)
	if err != nil {
		t.Fatalf("LoadRaw: %v", err)
	}
	// LoadRaw returns the on-disk view: ApiKey should still be the enc: form,
	// NOT the decrypted plaintext. (It unmarshals the TOML as-is.)
	if len(raw.ASRs) != 1 {
		t.Fatalf("raw providers = %d, want 1", len(raw.ASRs))
	}
	if raw.ASRs[0].ApiKey == "sk-raw-test" {
		// If it equaled plaintext, LoadRaw would be silently decrypting — a bug.
		t.Errorf("LoadRaw appears to have decrypted: ApiKey = %q", raw.ASRs[0].ApiKey)
	}
	if !strings.HasPrefix(raw.ASRs[0].ApiKey, "enc:") {
		t.Errorf("LoadRaw ApiKey = %q, want enc: prefix", raw.ASRs[0].ApiKey)
	}
}
