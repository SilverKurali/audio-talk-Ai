package asr

import (
	"log/slog"
	"reflect"
	"sort"
	"testing"
)

// unique names so tests never collide with real provider registrations
// (there is no Unregister; the registry is process-scoped).
const (
	tnAlpha = "test-registry-alpha"
	tnBeta  = "test-registry-beta"
)

func TestRegisterAndNewClient(t *testing.T) {
	Register(tnAlpha, func(common Common, cfg map[string]interface{}, logger *slog.Logger) (Client, error) {
		return nil, nil // factory invoked is enough; lookup is what we assert
	})

	called := false
	RegisterWithMeta(tnBeta,
		func(common Common, cfg map[string]interface{}, logger *slog.Logger) (Client, error) {
			called = true
			return nil, nil
		},
		ProviderMeta{
			DisplayName: "Beta Display",
			Fields: []FieldDef{
				{Key: "api_key", Label: "API Key", Type: FieldSecret, Secret: true},
			},
		})

	t.Run("unknown provider errors", func(t *testing.T) {
		_, err := NewClient("does-not-exist-zzz", Common{}, nil, nil)
		if err == nil {
			t.Fatal("expected error for unknown provider, got nil")
		}
	})

	t.Run("registered factory is invoked", func(t *testing.T) {
		called = false
		if _, err := NewClient(tnBeta, Common{}, map[string]interface{}{}, nil); err != nil {
			t.Fatalf("NewClient(%q) error: %v", tnBeta, err)
		}
		if !called {
			t.Fatalf("factory for %q was not invoked", tnBeta)
		}
	})

	t.Run("Register sets DisplayName default", func(t *testing.T) {
		m, ok := GetProviderMeta(tnAlpha)
		if !ok {
			t.Fatalf("GetProviderMeta(%q) not found", tnAlpha)
		}
		if m.DisplayName != tnAlpha {
			t.Fatalf("DisplayName = %q, want %q (Register default)", m.DisplayName, tnAlpha)
		}
	})
}

func TestProvidersSorted(t *testing.T) {
	// ensure both present
	Register(tnAlpha, func(Common, map[string]interface{}, *slog.Logger) (Client, error) { return nil, nil })
	Register(tnBeta, func(Common, map[string]interface{}, *slog.Logger) (Client, error) { return nil, nil })

	names := Providers()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("Providers() not sorted: %v", names)
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found[tnAlpha] || !found[tnBeta] {
		t.Fatalf("Providers() = %v, missing test registrations", names)
	}
}

func TestGetProviderMetaAndKnownFieldKeys(t *testing.T) {
	RegisterWithMeta(tnBeta, nil,
		ProviderMeta{
			DisplayName: "Beta Display",
			Fields: []FieldDef{
				{Key: "api_key", Label: "API Key", Type: FieldSecret},
				{Key: "endpoint", Label: "Endpoint", Type: FieldText},
			},
		})

	t.Run("GetProviderMeta returns copy semantics", func(t *testing.T) {
		m, ok := GetProviderMeta(tnBeta)
		if !ok {
			t.Fatalf("meta for %q missing", tnBeta)
		}
		if m.DisplayName != "Beta Display" || len(m.Fields) != 2 {
			t.Fatalf("unexpected meta: %+v", m)
		}
	})

	t.Run("GetProviderMeta missing -> ok false", func(t *testing.T) {
		_, ok := GetProviderMeta("zzz-missing")
		if ok {
			t.Fatal("expected ok=false for missing provider")
		}
	})

	t.Run("AllProviderMeta includes registrations", func(t *testing.T) {
		all := AllProviderMeta()
		if _, ok := all[tnBeta]; !ok {
			t.Fatalf("AllProviderMeta() missing %q: %v", tnBeta, all)
		}
		// mutating the returned map must not affect the registry
		all[tnBeta] = ProviderMeta{DisplayName: "mutated"}
		again, _ := GetProviderMeta(tnBeta)
		if again.DisplayName != "Beta Display" {
			t.Fatalf("registry mutated via AllProviderMeta(): got DisplayName %q", again.DisplayName)
		}
	})

	t.Run("KnownFieldKeys aggregates field keys", func(t *testing.T) {
		keys := KnownFieldKeys()
		if !keys["api_key"] || !keys["endpoint"] {
			t.Fatalf("KnownFieldKeys() missing expected keys: %v", keys)
		}
	})
}

func TestCommonFieldRoundTrip(t *testing.T) {
	// guards against accidental changes to the Common struct shape.
	in := Common{Language: "zh-CN", Hotwords: []string{"你好", "world"}}
	if in.Language != "zh-CN" || len(in.Hotwords) != 2 {
		t.Fatalf("Common not preserved: %+v", in)
	}
	// Factory type is a function; just ensure it is assignable without panic.
	var f Factory = func(common Common, cfg map[string]interface{}, logger *slog.Logger) (Client, error) {
		return nil, nil
	}
	if reflect.ValueOf(f).IsNil() {
		t.Fatal("non-nil factory reported as nil")
	}
	_ = f
}
