//go:build (!linux && !darwin && !windows) || no_x11

package overlay

import (
	"fmt"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

func newBackend(cfg config.OverlayConfig) (backend, error) {
	return nil, fmt.Errorf("overlay backend is not implemented for this platform")
}
