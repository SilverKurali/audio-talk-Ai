//go:build linux

package doctor

import (
	"strings"
	"testing"
)

// clearBackendEnv blanks every env var detectBackend consults so each subtest
// starts from a known state.
func clearBackendEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"JUST_TALK_BACKEND", "WAYLAND_DISPLAY", "XDG_SESSION_TYPE", "DISPLAY",
		"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "KDE_SESSION_VERSION"} {
		t.Setenv(k, "")
	}
}

func TestDetectBackend(t *testing.T) {
	cases := []struct {
		name string
		setup func(t *testing.T)
		want  string
	}{
		{
			"explicit arg wins",
			func(t *testing.T) {
				t.Setenv("JUST_TALK_BACKEND", "wayland")
				t.Setenv("WAYLAND_DISPLAY", "wayland-0")
				t.Setenv("DISPLAY", ":0")
			},
			"x11", // the function argument, not env
		},
		{
			"JUST_TALK_BACKEND env",
			func(t *testing.T) { t.Setenv("JUST_TALK_BACKEND", "wayland") },
			"wayland",
		},
		{
			"WAYLAND_DISPLAY set",
			func(t *testing.T) { t.Setenv("WAYLAND_DISPLAY", "wayland-1") },
			"wayland",
		},
		{
			"XDG_SESSION_TYPE=wayland",
			func(t *testing.T) { t.Setenv("XDG_SESSION_TYPE", "wayland") },
			"wayland",
		},
		{
			"DISPLAY -> x11",
			func(t *testing.T) { t.Setenv("DISPLAY", ":0") },
			"x11",
		},
		{
			"nothing set -> unknown",
			func(t *testing.T) {},
			"unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearBackendEnv(t)
			arg := ""
			if tc.name == "explicit arg wins" {
				arg = "x11"
			}
			tc.setup(t)
			if got := detectBackend(arg); got != tc.want {
				t.Fatalf("detectBackend(%q) = %q, want %q", arg, got, tc.want)
			}
		})
	}
}

func TestWaylandChosenOverX11(t *testing.T) {
	clearBackendEnv(t)
	// both WAYLAND_DISPLAY and DISPLAY set -> wayland takes priority
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":0")
	if got := detectBackend(""); got != "wayland" {
		t.Fatalf("detectBackend = %q, want wayland (wayland beats x11)", got)
	}
}

func TestDesktopInfo(t *testing.T) {
	clearBackendEnv(t)
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	t.Setenv("KDE_SESSION_VERSION", "")
	got := desktopInfo()
	if !strings.Contains(got, "XDG_CURRENT_DESKTOP=GNOME") {
		t.Errorf("desktopInfo = %q, want XDG_CURRENT_DESKTOP=GNOME included", got)
	}
	// empty vars must be omitted
	if strings.Contains(got, "KDE_SESSION_VERSION=") {
		t.Errorf("desktopInfo should omit empty vars: %q", got)
	}
}

func TestIsKDEPlasma(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T)
		want  bool
	}{
		{"KDE current desktop", func(t *testing.T) { t.Setenv("XDG_CURRENT_DESKTOP", "KDE") }, true},
		{"plasma session", func(t *testing.T) { t.Setenv("DESKTOP_SESSION", "plasma") }, true},
		{"KDE_SESSION_VERSION set", func(t *testing.T) { t.Setenv("KDE_SESSION_VERSION", "6") }, true},
		{"GNOME -> false", func(t *testing.T) { t.Setenv("XDG_CURRENT_DESKTOP", "GNOME") }, false},
		{"nothing -> false", func(t *testing.T) {}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearBackendEnv(t)
			tc.setup(t)
			if got := isKDEPlasma(); got != tc.want {
				t.Errorf("isKDEPlasma = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnvDetail(t *testing.T) {
	clearBackendEnv(t)
	t.Setenv("FOO", "bar")
	t.Setenv("EMPTY", "")
	got := envDetail("FOO", "EMPTY", "MISSING")
	// set var shows its value; unset/empty show <empty>
	if !strings.Contains(got, "FOO=bar") {
		t.Errorf("envDetail missing FOO=bar: %q", got)
	}
	if !strings.Contains(got, "EMPTY=<empty>") {
		t.Errorf("envDetail should show <empty> for empty var: %q", got)
	}
	if !strings.Contains(got, "MISSING=<empty>") {
		t.Errorf("envDetail should show <empty> for unset var: %q", got)
	}
}
