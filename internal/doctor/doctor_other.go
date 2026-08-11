//go:build !linux && !darwin && !windows

package doctor

import (
	"runtime"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

func runPlatform(cfg *config.Config, backend string) Report {
	return Report{
		Platform: runtime.GOOS,
		Backend:  backend,
		Checks: []Check{
			{
				Name:     "平台支持",
				OK:       false,
				Severity: Required,
				Detail:   runtime.GOOS + " 暂未实现",
				Fix:      "当前版本支持 Linux（Wayland/X11）、macOS 和 Windows；其他平台暂未实现。",
			},
		},
	}
}
