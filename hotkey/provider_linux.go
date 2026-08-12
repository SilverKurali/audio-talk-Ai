//go:build linux

package hotkey

import (
	"fmt"
	"log/slog"
	"os"
)

// NewProvider detects the display server and returns the appropriate provider.
//
// Detection order:
//  1. If XDG_SESSION_TYPE=wayland or WAYLAND_DISPLAY is set, use Wayland provider (evdev)
//  2. Otherwise, use X11 provider (XGrabKey)
//
// Both backends can also be explicitly selected via the JUST_TALK_BACKEND
// environment variable: "x11" or "wayland".
//
// The Wayland detection mirrors the logic in doctor.detectBackend,
// autotype.isWaylandSession, and clipboard.candidates so that the hotkey
// backend never disagrees with the rest of the stack when only one of the two
// Wayland-related env vars is set (e.g. WAYLAND_DISPLAY set but
// XDG_SESSION_TYPE empty, common under custom launchers).
func NewProvider() (Provider, error) {
	logger := slog.Default().With("platform", "linux")

	// Allow explicit override
	backend := os.Getenv("JUST_TALK_BACKEND")
	if backend == "" {
		if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland" {
			backend = "wayland"
		} else {
			backend = "x11"
		}
	}

	switch backend {
	case "x11":
		logger.Info("selected X11 backend")
		return newX11Provider()
	case "wayland":
		logger.Info("selected Wayland/evdev backend")
		return newWaylandProvider()
	default:
		return nil, fmt.Errorf("unknown backend: %s (expected 'x11' or 'wayland')", backend)
	}
}
