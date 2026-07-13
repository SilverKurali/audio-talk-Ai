package asr

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
	metas     = map[string]ProviderMeta{}
)

// Register registers an ASR provider factory by name (backward compatible).
// Providers should prefer RegisterWithMeta to also supply UI metadata.
func Register(name string, f Factory) {
	RegisterWithMeta(name, f, ProviderMeta{DisplayName: name})
}

// RegisterWithMeta registers a provider factory AND its metadata.
func RegisterWithMeta(name string, f Factory, meta ProviderMeta) {
	mu.Lock()
	defer mu.Unlock()
	factories[name] = f
	metas[name] = meta
}

// NewClient creates a new ASR client for the given provider name.
func NewClient(provider string, common Common, cfg map[string]interface{}, logger *slog.Logger) (Client, error) {
	mu.RLock()
	f, ok := factories[provider]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown ASR provider %q", provider)
	}
	return f(common, cfg, logger)
}

// Providers returns the sorted list of registered provider names.
func Providers() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// GetProviderMeta returns metadata for a named provider.
func GetProviderMeta(name string) (ProviderMeta, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := metas[name]
	return m, ok
}

// AllProviderMeta returns metadata for all registered providers, keyed by name.
func AllProviderMeta() map[string]ProviderMeta {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]ProviderMeta, len(metas))
	for k, v := range metas {
		out[k] = v
	}
	return out
}

// KnownFieldKeys returns the set of all field keys declared across all providers.
// Used by config loading to identify unknown (Extra) keys.
func KnownFieldKeys() map[string]bool {
	mu.RLock()
	defer mu.RUnlock()
	keys := make(map[string]bool)
	for _, m := range metas {
		for _, fd := range m.Fields {
			keys[fd.Key] = true
		}
	}
	return keys
}
