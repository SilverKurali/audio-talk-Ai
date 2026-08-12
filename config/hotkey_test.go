package config

import (
	"testing"

	"gitee.com/AY77-OP/audio-talk-ai/hotkey"
)

func TestParseHotkey(t *testing.T) {
	mods := func(m hotkey.Modifier) hotkey.Combo { return hotkey.Combo{Mods: m, Key: hotkey.KeyNone} }

	cases := []struct {
		name string
		in   string
		want hotkey.Combo
	}{
		{"function key", "F9", hotkey.Combo{Mods: hotkey.ModNone, Key: hotkey.KeyF9}},
		{"ctrl+shift+f9", "Ctrl+Shift+F9", hotkey.Combo{Mods: hotkey.ModCtrl | hotkey.ModShift, Key: hotkey.KeyF9}},
		{"alt+f4", "Alt+F4", hotkey.Combo{Mods: hotkey.ModAlt, Key: hotkey.KeyF4}},
		{"modifier only ctrl", "Ctrl", mods(hotkey.ModCtrl)},
		{"modifier only ctrl+shift", "Ctrl+Shift", mods(hotkey.ModCtrl | hotkey.ModShift)},
		{"modifier only super alias", "Cmd", mods(hotkey.ModSuper)},
		{"text key lower", "a", hotkey.Combo{Mods: hotkey.ModNone, Key: hotkey.KeyA}},
		{"option+cmd+space", "Option+Cmd+Space", hotkey.Combo{Mods: hotkey.ModAlt | hotkey.ModSuper, Key: hotkey.KeySpace}},
		{"punctuation backtick", "`", hotkey.Combo{Mods: hotkey.ModNone, Key: hotkey.KeyBacktick}},
		{"punctuation minus", "-", hotkey.Combo{Mods: hotkey.ModNone, Key: hotkey.KeyMinus}},
		{"punctuation equal", "=", hotkey.Combo{Mods: hotkey.ModNone, Key: hotkey.KeyEqual}},
		{"whitespace trimmed", "   F9   ", hotkey.Combo{Mods: hotkey.ModNone, Key: hotkey.KeyF9}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseHotkey(tc.in)
			if err != nil {
				t.Fatalf("ParseHotkey(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseHotkey(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseHotkeyErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"unknown key", "foo+bar"},
		{"multiple keys", "A+B"},
		{"trailing empty part", "F9+"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseHotkey(tc.in); err == nil {
				t.Fatalf("ParseHotkey(%q) expected error, got nil", tc.in)
			}
		})
	}
}

func TestParseHotkeyRoundTrip(t *testing.T) {
	// parse -> String -> parse must yield the same combo (format-agnostic).
	inputs := []string{
		"F9", "Ctrl+Shift+F9", "Alt+F4", "Ctrl", "Ctrl+Shift",
		"Cmd", "a", "Option+Cmd+Space", "-", "=", "`",
		"Ctrl+Alt+Delete", "Shift+Up",
	}
	for _, in := range inputs {
		c1, err := ParseHotkey(in)
		if err != nil {
			t.Fatalf("ParseHotkey(%q) error: %v", in, err)
		}
		s := c1.String()
		c2, err := ParseHotkey(s)
		if err != nil {
			t.Fatalf("round-trip: ParseHotkey(%q) [from String %q] error: %v", in, s, err)
		}
		if c1 != c2 {
			t.Fatalf("round-trip mismatch for %q: %+v -> %q -> %+v", in, c1, s, c2)
		}
	}
}

func TestParseHotkeys(t *testing.T) {
	t.Run("multiple valid", func(t *testing.T) {
		combos, err := ParseHotkeys([]string{"F9", "Ctrl+Shift+F9"})
		if err != nil {
			t.Fatalf("ParseHotkeys error: %v", err)
		}
		if len(combos) != 2 || combos[0].Key != hotkey.KeyF9 || combos[1].Key != hotkey.KeyF9 {
			t.Fatalf("ParseHotkeys = %+v", combos)
		}
	})

	t.Run("propagates first error", func(t *testing.T) {
		_, err := ParseHotkeys([]string{"F9", "badkey+x"})
		if err == nil {
			t.Fatal("expected error from invalid entry")
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		combos, err := ParseHotkeys(nil)
		if err != nil || combos != nil {
			t.Fatalf("ParseHotkeys(nil) = (%+v,%v), want (nil,nil)", combos, err)
		}
	})
}
