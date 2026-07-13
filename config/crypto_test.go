package config

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	samples := []string{
		"",
		"my-secret-appkey-12345",
		"with/slash:and=plus+special chars 中文",
	}
	for _, plain := range samples {
		enc, err := encryptString([]byte(plain), key)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		if plain == "" {
			if enc != "" {
				t.Fatalf("empty plaintext should stay empty, got %q", enc)
			}
			continue
		}
		if !strings.HasPrefix(enc, encPrefix) {
			t.Fatalf("encrypt %q: missing enc prefix: %q", plain, enc)
		}
		got, err := decryptString(enc, key)
		if err != nil {
			t.Fatalf("decrypt round-trip failed for %q: %v", plain, err)
		}
		if got != plain {
			t.Fatalf("round-trip mismatch: got %q want %q", got, plain)
		}
	}
}

func TestDecryptFieldNoPrefix(t *testing.T) {
	key := make([]byte, 32)
	p := "plaintext-not-encrypted"
	if err := decryptField(&p, key); err != nil {
		t.Fatalf("decryptField should pass through non-prefixed value: %v", err)
	}
	if p != "plaintext-not-encrypted" {
		t.Fatalf("value mutated: %q", p)
	}
}
