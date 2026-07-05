package asr

import (
	"context"
	"log/slog"
)

// Client is a streaming ASR session.
type Client interface {
	Connect(ctx context.Context) error
	SendAudio(ctx context.Context, pcm []byte, isLast bool) error
	Results() <-chan Result
	Done() <-chan struct{}
	Final() <-chan struct{}
	LastText() string
	ReceiveLoop(ctx context.Context)
	Close() error
}

// Result is one ASR recognition output.
type Result struct {
	Text    string
	IsFinal bool
	Error   error
}

// Common holds provider-agnostic session settings.
type Common struct {
	Language string
	Hotwords []string
}

// Factory creates a Client for a provider.
// providerCfg carries provider-specific settings as key-value pairs.
type Factory func(common Common, providerCfg map[string]interface{}, logger *slog.Logger) (Client, error)
