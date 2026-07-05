//go:build linux && !no_x11

package overlay

import (
	"fmt"
	"os"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

func newBackend(cfg config.OverlayConfig) (backend, error) {
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != "" {
		if b, err := newWaylandBackend(cfg); err == nil {
			return b, nil
		} else if os.Getenv("DISPLAY") == "" {
			return nil, err
		}
	}
	if os.Getenv("DISPLAY") != "" {
		return newX11Backend(cfg)
	}
	return nil, fmt.Errorf("no supported display server found")
}
