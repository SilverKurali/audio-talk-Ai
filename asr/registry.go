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
)

// Register registers an ASR provider factory by name.
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[name] = f
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
