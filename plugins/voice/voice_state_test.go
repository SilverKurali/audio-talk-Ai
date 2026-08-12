package voice

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"gitee.com/AY77-OP/audio-talk-ai/config"
)

// stopTimerLocked stops any pending stop timer so it cannot fire during the
// rest of the test (the timer callback detaches/finishes recording).
func stopTimerLocked(p *VoicePlugin) {
	p.mu.Lock()
	if p.stopTimer != nil {
		p.stopTimer.Stop()
		p.stopTimer = nil
	}
	p.mu.Unlock()
}

// ---- pure state-flip methods ----

func TestStartStopDelay(t *testing.T) {
	p := newTestPlugin(t)
	p.mu.Lock()
	p.recording = true
	p.stopDelayMs = 600000 // never fire during the test
	p.mu.Unlock()

	p.startStopDelay()

	p.mu.Lock()
	stopping, userStopped, holdReleased := p.stopping, p.userStopped, p.holdReleased
	timerSet := p.stopTimer != nil
	p.mu.Unlock()
	stopTimerLocked(p)

	if !stopping {
		t.Error("stopping should be true after startStopDelay")
	}
	if !userStopped {
		t.Error("userStopped should be true after startStopDelay")
	}
	if holdReleased {
		t.Error("holdReleased should be cleared")
	}
	if !timerSet {
		t.Error("stopTimer should be armed")
	}
}

func TestStartStopDelayNoOpWhenNotRecording(t *testing.T) {
	p := newTestPlugin(t)
	// recording is false -> startStopDelay returns early
	p.startStopDelay()
	p.mu.Lock()
	stopping := p.stopping
	p.mu.Unlock()
	if stopping {
		t.Error("startStopDelay should be a no-op when not recording")
	}
}

func TestCancelStopDelay(t *testing.T) {
	p := newTestPlugin(t)
	p.mu.Lock()
	p.recording = true
	p.stopping = true
	p.userStopped = true
	p.stopDelayMs = 600000
	p.stopTimer = time.AfterFunc(time.Hour, func() {})
	p.mu.Unlock()

	p.cancelStopDelay()

	p.mu.Lock()
	stopping := p.stopping
	timerCleared := p.stopTimer == nil
	p.mu.Unlock()
	if stopping {
		t.Error("stopping should be false after cancelStopDelay")
	}
	if !timerCleared {
		t.Error("stopTimer should be cleared")
	}
}

func TestMarkHoldReleasedTriggersStop(t *testing.T) {
	p := newTestPlugin(t)
	p.mu.Lock()
	p.recording = true
	p.stopping = false
	p.stopDelayMs = 600000
	p.mu.Unlock()

	p.markHoldReleased()

	p.mu.Lock()
	holdReleased, stopping := p.holdReleased, p.stopping
	p.mu.Unlock()
	stopTimerLocked(p)

	// markHoldReleased arms the stop delay, and startStopDelay clears
	// holdReleased (voice.go) — so the net state is stopping=true, holdReleased=false.
	if !stopping {
		t.Error("markHoldReleased should start the stop delay when recording && !stopping")
	}
	if holdReleased {
		t.Error("holdReleased should be cleared once the stop delay is armed")
	}
}

func TestMarkHoldReleasedNoOpWhenStopping(t *testing.T) {
	p := newTestPlugin(t)
	p.mu.Lock()
	p.recording = true
	p.stopping = true // already stopping -> should not re-arm
	p.stopDelayMs = 600000
	p.mu.Unlock()

	p.markHoldReleased()

	p.mu.Lock()
	timerSet := p.stopTimer != nil
	p.mu.Unlock()
	stopTimerLocked(p)
	if timerSet {
		t.Error("markHoldReleased should not arm a new timer when already stopping")
	}
}

func TestClearHoldReleased(t *testing.T) {
	p := newTestPlugin(t)
	p.mu.Lock()
	p.holdReleased = true
	p.mu.Unlock()
	p.clearHoldReleased()
	p.mu.Lock()
	got := p.holdReleased
	p.mu.Unlock()
	if got {
		t.Error("holdReleased should be cleared")
	}
}

// ---- connectASR via the injected asrFactory seam ----

func setupRecording(p *VoicePlugin, rec *Recorder, sessionGen uint64) {
	p.mu.Lock()
	p.recording = true
	p.recorder = rec
	p.sessionGen = sessionGen
	p.mu.Unlock()
}

func TestConnectASRFactoryError(t *testing.T) {
	p := newTestPlugin(t)
	p.asrFactory = func(string, asr.Common, map[string]interface{}, *slog.Logger) (asr.Client, error) {
		return nil, errors.New("factory boom")
	}
	rec := &Recorder{logger: p.logger}
	setupRecording(p, rec, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.connectASR(ctx, cancel, 1, 1, rec, config.ASRProviderConfig{Name: "t", Type: "openai-whisper"}, asr.Common{})

	p.mu.Lock()
	recording := p.recording
	lastError := p.lastError
	p.mu.Unlock()
	if recording {
		t.Error("recording should be reset to false on factory error")
	}
	if lastError == "" {
		t.Error("an error should be published on factory failure")
	}
}

func TestConnectASRConnectError(t *testing.T) {
	p := newTestPlugin(t)
	mock := newMockASR()
	mock.connectErr = errors.New("dial failed")
	p.asrFactory = func(string, asr.Common, map[string]interface{}, *slog.Logger) (asr.Client, error) {
		return mock, nil
	}
	rec := &Recorder{logger: p.logger}
	setupRecording(p, rec, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.connectASR(ctx, cancel, 1, 1, rec, config.ASRProviderConfig{Name: "t", Type: "openai-whisper"}, asr.Common{})

	p.mu.Lock()
	recording := p.recording
	lastError := p.lastError
	p.mu.Unlock()
	if recording {
		t.Error("recording should be reset on Connect error")
	}
	if lastError == "" {
		t.Error("an error should be published on Connect failure")
	}
}

func TestConnectASRSuccessAssignsClient(t *testing.T) {
	p := newTestPlugin(t)
	mock := newMockASR()
	p.asrFactory = func(string, asr.Common, map[string]interface{}, *slog.Logger) (asr.Client, error) {
		return mock, nil
	}
	rec := &Recorder{logger: p.logger}
	setupRecording(p, rec, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // tears down ReceiveLoop/streamAudio goroutines
	p.connectASR(ctx, cancel, 1, 1, rec, config.ASRProviderConfig{Name: "t", Type: "openai-whisper"}, asr.Common{})

	p.mu.Lock()
	client := p.asrClient
	p.mu.Unlock()
	if client != mock {
		t.Error("asrClient should be the mock on successful connect")
	}
}

func TestConnectASRStaleSessionDropped(t *testing.T) {
	// A newer session has started (sessionGen advanced) by the time Connect
	// completes. The stale client must be closed and NOT assigned.
	p := newTestPlugin(t)
	mock := newMockASR()
	p.asrFactory = func(string, asr.Common, map[string]interface{}, *slog.Logger) (asr.Client, error) {
		return mock, nil
	}
	rec := &Recorder{logger: p.logger}
	// current session is gen 2, but this connectASR is for the old gen 1
	setupRecording(p, rec, 2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.connectASR(ctx, cancel, 1, 1, rec, config.ASRProviderConfig{Name: "t", Type: "openai-whisper"}, asr.Common{})

	p.mu.Lock()
	client := p.asrClient
	p.mu.Unlock()
	if client != nil {
		t.Error("stale-session client must not be assigned to asrClient")
	}
	if !mock.Closed() {
		t.Error("stale-session client must be closed")
	}
}

// ---- startRecording error paths (deterministic on headless CI) ----

func TestStartRecordingNoProviders(t *testing.T) {
	p := newTestPlugin(t)
	// no ASRs and no voice keys -> ResolveASRProviders returns nil
	p.cfg.ASRs = nil
	p.cfg.Voice.AppKey = ""

	p.startRecording()

	p.mu.Lock()
	recording := p.recording
	lastError := p.lastError
	p.mu.Unlock()
	if recording {
		t.Error("should not be recording with no providers configured")
	}
	if lastError == "" {
		t.Error("should publish an error when no ASR provider is configured")
	}
}

func TestStartRecordingRecorderStartFails(t *testing.T) {
	// The empty-PATH trick only forces a recorder-start failure on platforms
	// whose recorder shells out (arecord on Linux, ffmpeg/sox on Windows).
	// macOS uses CoreAudio via cgo and ignores PATH, so its failure path needs
	// real hardware — out of scope for CI (see AGENTS.md platform notes).
	if runtime.GOOS == "darwin" {
		t.Skip("recorder uses CoreAudio on macOS; start-failure path needs hardware")
	}
	// Empty PATH => arecord/ffmpeg not found => recorder.Start fails deterministically.
	t.Setenv("PATH", t.TempDir())
	p := newTestPlugin(t)
	p.cfg.ASRs = []config.ASRProviderConfig{{Name: "p", Type: "openai-whisper", Default: true}}
	// factory is never reached because the recorder fails first; leave it nil.

	p.startRecording()

	p.mu.Lock()
	recording := p.recording
	lastError := p.lastError
	p.mu.Unlock()
	if recording {
		t.Error("should not be recording when recorder fails to start")
	}
	if lastError == "" {
		t.Error("should publish a recorder-start error")
	}
}
