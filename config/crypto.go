package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// encPrefix marks a config value that is stored encrypted at rest.
const encPrefix = "enc:"

// keyPath returns the per-user key file used to encrypt secrets on disk.
func keyPath() string {
	dir := keyDir()
	return filepath.Join(dir, "key")
}

// keyDir mirrors the config directory resolution (XDG_CONFIG_HOME or ~/.config).
func keyDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "audio-talk-ai")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "audio-talk-ai")
	}
	return ".config/audio-talk-ai"
}

// loadKey returns the AES-256 key, creating it (readable only by the user) if missing.
func loadKey() ([]byte, error) {
	p := keyPath()
	if data, err := os.ReadFile(p); err == nil && len(data) >= 32 {
		return data[:32], nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return nil, fmt.Errorf("create key dir: %w", err)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(key); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	return key, nil
}

// KeyFilePath returns the on-disk path of the encryption key file.
func KeyFilePath() string {
	return keyPath()
}

// HasEncryptedSecrets reports whether any credential is already stored
// encrypted (enc: prefix).
func HasEncryptedSecrets(c *Config) bool {
	for i := range c.ASRs {
		p := c.ASRs[i]
		if strings.HasPrefix(p.AppKey, encPrefix) || strings.HasPrefix(p.AccessKey, encPrefix) ||
			strings.HasPrefix(p.ResourceID, encPrefix) || strings.HasPrefix(p.ApiKey, encPrefix) ||
			strings.HasPrefix(p.AppID, encPrefix) || strings.HasPrefix(p.ApiSecret, encPrefix) ||
			strings.HasPrefix(p.DWA, encPrefix) {
			return true
		}
	}
	return strings.HasPrefix(c.Voice.AppKey, encPrefix) ||
		strings.HasPrefix(c.Voice.AccessKey, encPrefix) ||
		strings.HasPrefix(c.Voice.ResourceID, encPrefix)
}

// encryptString encrypts plaintext with AES-256-GCM and returns "enc:<base64>".
// An empty plaintext is returned unchanged (nothing to encrypt).
func encryptString(plain, key []byte) (string, error) {
	if len(plain) == 0 {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, plain, nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// decryptString reverses encryptString. An empty value is returned unchanged.
func decryptString(enc string, key []byte) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(enc, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("stored secret too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret (wrong key?): %w", err)
	}
	return string(pt), nil
}
