package hotkey

import (
	"testing"
	"time"
)

func TestStandardComboKeyUpFiresWhenModifierReleasesFirst(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 4)
	combo := Combo{Mods: ModAlt, Key: KeyR}
	tracker.Watch(combo, ch)

	events := tracker.KeyDown(KeyAlt, time.Now())
	if len(events) != 0 {
		t.Fatalf("Alt down emitted %d events, want 0", len(events))
	}

	events = tracker.KeyDown(KeyR, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyDown {
		t.Fatalf("R down emitted %#v, want one KeyDown for %s", events, combo)
	}

	events = tracker.KeyUp(KeyAlt, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyUp {
		t.Fatalf("Alt up emitted %#v, want one KeyUp for %s", events, combo)
	}

	events = tracker.KeyUp(KeyR, time.Now())
	if len(events) != 0 {
		t.Fatalf("R up emitted %d events, want 0", len(events))
	}
}

func TestStandardComboKeyDownFiresWhenModifierPressedAfterKey(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 4)
	combo := Combo{Mods: ModAlt, Key: KeyR}
	tracker.Watch(combo, ch)

	events := tracker.KeyDown(KeyR, time.Now())
	if len(events) != 0 {
		t.Fatalf("R down emitted %d events, want 0", len(events))
	}

	events = tracker.KeyDown(KeyAlt, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyDown {
		t.Fatalf("Alt down emitted %#v, want one KeyDown for %s", events, combo)
	}

	events = tracker.KeyUp(KeyR, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyUp {
		t.Fatalf("R up emitted %#v, want one KeyUp for %s", events, combo)
	}

	events = tracker.KeyUp(KeyAlt, time.Now())
	if len(events) != 0 {
		t.Fatalf("Alt up emitted %d events, want 0", len(events))
	}
}

// TestKeyDownAutoRepeatIgnored verifies that keyboard auto-repeat (typematic),
// where a held key keeps producing KeyDown events (~30 Hz on Windows
// WH_KEYBOARD_LL), is de-duplicated. A solo hotkey must fire exactly once per
// physical press regardless of how many repeated KeyDown events arrive.
func TestKeyDownAutoRepeatIgnored(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 4)
	combo := Combo{Key: KeyF9} // key-only solo hotkey (toggle single key)
	tracker.Watch(combo, ch)

	// First physical press fires.
	first := tracker.KeyDown(KeyF9, time.Now())
	if len(first) != 1 || first[0].Type != KeyDown || first[0].Combo != combo {
		t.Fatalf("first KeyDown emitted %#v, want one KeyDown for %s", first, combo)
	}

	// Auto-repeat must not re-fire.
	for i := 0; i < 4; i++ {
		repeat := tracker.KeyDown(KeyF9, time.Now())
		if repeat != nil {
			t.Fatalf("auto-repeat KeyDown #%d emitted %#v, want nil", i+2, repeat)
		}
	}

	// State must still indicate pressed so the eventual KeyUp clears it.
	if !tracker.IsPressed(KeyF9) {
		t.Fatalf("IsPressed(F9) = false after auto-repeat, want true")
	}
}

// TestKeyOnlySoloKeyUpReliableAfterLongPress verifies that after a long
// auto-repeated press, the single KeyUp still fires reliably and clears the
// solo combo. This is the hold-mode regression: previously the repeated
// KeyDowns flooded the event channel and the KeyUp was dropped, leaving
// recording stuck on.
func TestKeyOnlySoloKeyUpReliableAfterLongPress(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 16)
	combo := Combo{Key: KeyF9}
	tracker.Watch(combo, ch)

	// Simulate a 5-second hold: one real KeyDown then many auto-repeats.
	down := tracker.KeyDown(KeyF9, time.Now())
	if len(down) != 1 {
		t.Fatalf("initial KeyDown emitted %d events, want 1", len(down))
	}
	for i := 0; i < 150; i++ { // ~5s at 30 Hz
		if e := tracker.KeyDown(KeyF9, time.Now()); e != nil {
			t.Fatalf("auto-repeat #%d emitted %#v, want nil", i+1, e)
		}
	}

	// Release must fire exactly one KeyUp and clear the solo combo.
	up := tracker.KeyUp(KeyF9, time.Now())
	if len(up) != 1 || up[0].Type != KeyUp || up[0].Combo != combo {
		t.Fatalf("KeyUp after long press emitted %#v, want one KeyUp for %s", up, combo)
	}
	if tracker.IsPressed(KeyF9) {
		t.Fatalf("IsPressed(F9) = true after KeyUp, want false")
	}
	if tracker.activeSoloCombos[combo] {
		t.Fatalf("activeSoloCombos still set after KeyUp")
	}

	// Re-press must work again (the dedup must not have permanently suppressed
	// the key — it only dedups while held).
	down2 := tracker.KeyDown(KeyF9, time.Now())
	if len(down2) != 1 || down2[0].Type != KeyDown {
		t.Fatalf("re-press KeyDown emitted %#v, want one KeyDown", down2)
	}
}

// TestToggleComboSingleFlipPerPress verifies that a key-only solo hotkey
// (toggle single key) produces exactly one KeyDown per press/release cycle.
// Previously auto-repeat flipped the toggle N times per physical press.
func TestToggleComboSingleFlipPerPress(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 16)
	combo := Combo{Key: KeyF9}
	tracker.Watch(combo, ch)

	var totalKeyDowns int
	presses := 3
	for i := 0; i < presses; i++ {
		ev := tracker.KeyDown(KeyF9, time.Now())
		totalKeyDowns += len(ev)
		// While "held", simulate auto-repeat.
		for r := 0; r < 10; r++ {
			totalKeyDowns += len(tracker.KeyDown(KeyF9, time.Now()))
		}
		// Release before the next press.
		tracker.KeyUp(KeyF9, time.Now())
	}

	if totalKeyDowns != presses {
		t.Fatalf("got %d total KeyDown events over %d presses (with auto-repeat), want %d",
			totalKeyDowns, presses, presses)
	}
}
