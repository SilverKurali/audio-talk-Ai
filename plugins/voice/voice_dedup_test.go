package voice

import (
	"testing"
	"time"
)

// claimSessionOutput is the single-dispatch guard for transcription output.
// AGENTS.md flags the Final()/Done() double-dispatch path as historically
// fragile — these tests pin the guard's behavior.

func TestClaimSessionOutput(t *testing.T) {
	p := newTestPlugin(t)

	t.Run("first claim succeeds", func(t *testing.T) {
		if !p.claimSessionOutput(100) {
			t.Fatal("first claim for session 100 should succeed")
		}
	})

	t.Run("second claim for same session fails", func(t *testing.T) {
		if p.claimSessionOutput(100) {
			t.Fatal("second claim for session 100 should be rejected")
		}
	})

	t.Run("different session succeeds", func(t *testing.T) {
		if !p.claimSessionOutput(101) {
			t.Fatal("claim for a different session should succeed")
		}
	})

	t.Run("canceled session rejected", func(t *testing.T) {
		p.mu.Lock()
		if p.canceledSessions == nil {
			p.canceledSessions = make(map[uint64]struct{})
		}
		p.canceledSessions[200] = struct{}{}
		p.mu.Unlock()
		if p.claimSessionOutput(200) {
			t.Fatal("canceled session should be rejected")
		}
	})
}

// runFinishDispatch invokes finishRecordingSession once and returns how many
// times the transcription callback fired.
func runFinishDispatch(t *testing.T, mock *mockASRClient, userStopped bool, sessionID uint64) int {
	t.Helper()
	p := newTestPlugin(t)
	count := 0
	SetTranscriptionCallback(func(text, provider string, duration time.Duration) {
		count++
		if text != "hello" {
			t.Errorf("callback text = %q, want hello", text)
		}
	})
	t.Cleanup(func() { onTranscription = nil })

	session := &recordingSession{
		sessionID:    sessionID,
		asrClient:    mock,
		userStopped:  userStopped,
		startedAt:    time.Now(),
		providerName: "test",
	}
	mock.SetLastText("hello")
	mock.SignalFinal()

	p.finishRecordingSession(session)
	return count
}

func TestFinishRecordingSessionDispatchesOnce(t *testing.T) {
	t.Run("via Final channel", func(t *testing.T) {
		mock := newMockASR()
		if n := runFinishDispatch(t, mock, true, 1); n != 1 {
			t.Fatalf("callback fired %d times, want 1", n)
		}
		if mock.Sends() < 1 {
			t.Error("expected final audio SendAudio call")
		}
	})

	t.Run("via Done channel (Final never signaled)", func(t *testing.T) {
		p := newTestPlugin(t)
		count := 0
		SetTranscriptionCallback(func(text, provider string, duration time.Duration) { count++ })
		t.Cleanup(func() { onTranscription = nil })

		mock := newMockASR()
		mock.SetLastText("hello")
		mock.SignalDone() // Done instead of Final
		session := &recordingSession{
			sessionID: 1, asrClient: mock, userStopped: true,
			startedAt: time.Now(), providerName: "test",
		}
		p.finishRecordingSession(session)
		if count != 1 {
			t.Fatalf("callback fired %d times via Done, want 1", count)
		}
	})
}

func TestFinishRecordingSessionNoDispatch(t *testing.T) {
	t.Run("empty text -> no dispatch", func(t *testing.T) {
		p := newTestPlugin(t)
		count := 0
		SetTranscriptionCallback(func(string, string, time.Duration) { count++ })
		t.Cleanup(func() { onTranscription = nil })

		mock := newMockASR()
		// lastText stays "" (never set)
		mock.SignalFinal()
		session := &recordingSession{sessionID: 1, asrClient: mock, userStopped: true, startedAt: time.Now()}
		p.finishRecordingSession(session)
		if count != 0 {
			t.Fatalf("empty text dispatched %d times, want 0", count)
		}
	})

	t.Run("userStopped false -> no dispatch", func(t *testing.T) {
		p := newTestPlugin(t)
		count := 0
		SetTranscriptionCallback(func(string, string, time.Duration) { count++ })
		t.Cleanup(func() { onTranscription = nil })

		mock := newMockASR()
		mock.SetLastText("hello")
		mock.SignalFinal()
		session := &recordingSession{sessionID: 1, asrClient: mock, userStopped: false, startedAt: time.Now()}
		p.finishRecordingSession(session)
		if count != 0 {
			t.Fatalf("userStopped=false dispatched %d times, want 0", count)
		}
	})
}

func TestFinishRecordingSessionNotDispatchedTwice(t *testing.T) {
	// Calling finish twice for the same session must not double-dispatch even
	// though both calls reach the Final/Done select.
	p := newTestPlugin(t)
	count := 0
	SetTranscriptionCallback(func(string, string, time.Duration) { count++ })
	t.Cleanup(func() { onTranscription = nil })

	mock := newMockASR()
	mock.SetLastText("hello")
	mock.SignalFinal()
	session := &recordingSession{
		sessionID: 1, asrClient: mock, userStopped: true,
		startedAt: time.Now(), providerName: "test",
	}

	p.finishRecordingSession(session)
	p.finishRecordingSession(session) // second finish must be a no-op for output

	if count != 1 {
		t.Fatalf("double finish dispatched %d times, want 1", count)
	}
}
