package drivers

import (
	"slices"
	"strings"
	"testing"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
)

// expectedProviders is the full set of provider names registered by the blank
// imports in drivers.go. Updating this test when adding/removing a provider is
// intentional — it guards against silent registration regressions.
var expectedProviders = []string{
	"doubao",
	"openai-realtime",
	"openai-whisper",
	"xfyun-iat",
	"xfyun-lfasr",
	"xfyun-lfasr-fast",
	"xfyun-lfasr-llm",
	"xfyun-rtasr",
	"xfyun-rtasr-std",
	"xfyun-spark",
	"xiaomi-mimo-asr",
	"xiaomi-mimo-asr-TokenPlan",
}

func TestAllProvidersRegistered(t *testing.T) {
	got := asr.Providers()
	if len(got) != len(expectedProviders) {
		t.Fatalf("registered %d providers, want %d: %v", len(got), len(expectedProviders), got)
	}
	for _, name := range expectedProviders {
		if !slices.Contains(got, name) {
			t.Errorf("provider %q not registered; got %v", name, got)
		}
	}
}

func TestAllProvidersHaveMetadata(t *testing.T) {
	for _, name := range expectedProviders {
		meta, ok := asr.GetProviderMeta(name)
		if !ok {
			t.Errorf("%q: no metadata", name)
			continue
		}
		if strings.TrimSpace(meta.DisplayName) == "" {
			t.Errorf("%q: empty DisplayName", name)
		}
		if len(meta.Fields) == 0 {
			t.Errorf("%q: no fields declared", name)
		}
	}
}

func TestKnownFieldKeysNonEmpty(t *testing.T) {
	keys := asr.KnownFieldKeys()
	if len(keys) == 0 {
		t.Fatal("KnownFieldKeys() empty; providers did not declare fields")
	}
	// Every provider needs credentials of some kind; api_key/app_id/access_key
	// are the common ones. At least one must appear.
	common := []string{"api_key", "app_id", "access_key", "app_key"}
	found := false
	for _, k := range common {
		if keys[k] {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("none of common credential keys %v present in %v", common, keys)
	}
}
