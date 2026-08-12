package voice

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"gitee.com/AY77-OP/audio-talk-ai/config"
	"gitee.com/AY77-OP/audio-talk-ai/engine"
	"gitee.com/AY77-OP/audio-talk-ai/hotkey"
)

// mockASRClient is a controllable asr.Client for testing the voice pipeline.
// Tests drive it via SetLastText / SignalFinal / SignalDone.
type mockASRClient struct {
	mu        sync.Mutex
	lastText  string
	sends     int
	closed    bool
	connectErr error
	final     chan struct{}
	done      chan struct{}
	results   chan asr.Result
}

func newMockASR() *mockASRClient {
	return &mockASRClient{
		final:   make(chan struct{}),
		done:    make(chan struct{}),
		results: make(chan asr.Result, 8),
	}
}

func (m *mockASRClient) Connect(ctx context.Context) error { return m.connectErr }
func (m *mockASRClient) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	m.mu.Lock()
	m.sends++
	m.mu.Unlock()
	return nil
}
func (m *mockASRClient) Results() <-chan asr.Result    { return m.results }
func (m *mockASRClient) Done() <-chan struct{}         { return m.done }
func (m *mockASRClient) Final() <-chan struct{}        { return m.final }
func (m *mockASRClient) LastText() string              { m.mu.Lock(); defer m.mu.Unlock(); return m.lastText }
func (m *mockASRClient) ReceiveLoop(ctx context.Context) { <-ctx.Done() }
func (m *mockASRClient) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}

// control API
func (m *mockASRClient) SetLastText(s string) { m.mu.Lock(); m.lastText = s; m.mu.Unlock() }
func (m *mockASRClient) Sends() int           { m.mu.Lock(); defer m.mu.Unlock(); return m.sends }
func (m *mockASRClient) Closed() bool         { m.mu.Lock(); defer m.mu.Unlock(); return m.closed }

// closeOnce closes ch if it is still open (idempotent, no double-close panic).
func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}

func (m *mockASRClient) SignalFinal() { closeOnce(m.final) }
func (m *mockASRClient) SignalDone()  { closeOnce(m.done) }

// stubEnv is a no-op engine.PluginEnv so plugins can be exercised without a
// real hotkey provider. RegisterHotkey/UnregisterHotkey succeed silently.
type stubEnv struct {
	logger *slog.Logger
	cfg    *config.Config
}

func (s *stubEnv) RegisterHotkey(combo hotkey.Combo, handler func(hotkey.Event)) error {
	return nil
}
func (s *stubEnv) RegisterHotkeyWithOptions(combo hotkey.Combo, opts hotkey.RegisterOptions, handler func(hotkey.Event)) error {
	return nil
}
func (s *stubEnv) UnregisterHotkey(combo hotkey.Combo) error { return nil }
func (s *stubEnv) Logger() *slog.Logger                      { return s.logger }
func (s *stubEnv) Config() *config.Config                    { return s.cfg }
func (s *stubEnv) Engine() *engine.Engine                    { return nil }

// newTestPlugin builds a VoicePlugin wired to a stub env, with output discarded
// and stats redirected to a temp dir. Callers set fields (recording, asrFactory,
// ...) and invoke the method under test.
func newTestPlugin(t *testing.T) *VoicePlugin {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	SetOutput(io.Discard)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()
	cfg.Voice.AutoSubmit = false // avoid autotype/clipboard side effects by default
	// finishRecordingSession spawns `go saveTUIStats` on dispatch, which writes
	// into stateDir. On Windows, t.TempDir's RemoveAll races that goroutine and
	// fails with "directory not empty", so wait for the write to settle first.
	// Registered after t.TempDir, this runs BEFORE TempDir's LIFO cleanup.
	t.Cleanup(func() { waitForStatsWrite(stateDir) })
	return &VoicePlugin{
		env:         &stubEnv{logger: logger, cfg: cfg},
		logger:      logger,
		cfg:         cfg,
		stopDelayMs: defaultStopDelayMs,
	}
}

// waitForStatsWrite blocks until a pending saveTUIStats goroutine has finished
// writing stats.json into stateDir, or returns quickly when no dispatch (hence
// no write) is expected. This keeps Windows t.TempDir cleanup race-free.
func waitForStatsWrite(stateDir string) {
	statsDir := filepath.Join(stateDir, "audio-talk-ai")
	statsFile := filepath.Join(statsDir, "stats.json")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(statsFile); err == nil && info.Size() > 0 {
			return // write complete, file handle closed -> safe to RemoveAll
		}
		// If the stats dir was never created, no dispatch happened: give the
		// scheduler a short grace, then bail (nothing to wait for).
		if _, err := os.Stat(statsDir); err != nil {
			time.Sleep(15 * time.Millisecond)
			if _, err := os.Stat(statsDir); err != nil {
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
}
