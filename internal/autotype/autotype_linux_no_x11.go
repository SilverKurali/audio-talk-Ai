//go:build linux && no_x11

package autotype

import (
	"fmt"
	"log/slog"
)

// pastePlatform is a stub for headless/no-X11 Linux builds (e.g. CI).
// Auto-submit requires X11 or Wayland, which are excluded under the no_x11 tag.
func pastePlatform(text string, logger *slog.Logger) error {
	return fmt.Errorf("autotype is not available in no_x11 builds")
}

func pasteMethod() string { return "linux/no_x11 (unavailable)" }

func isWaylandSession() bool { return false }