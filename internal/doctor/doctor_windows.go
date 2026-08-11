//go:build windows

package doctor

import (
	"fmt"
	"strings"

	"gitee.com/AY77-OP/audio-talk-ai/config"
	"golang.org/x/sys/windows"
)

var (
	doctorWinMM                = windows.NewLazySystemDLL("winmm.dll")
	doctorProcWaveInGetNumDevs = doctorWinMM.NewProc("waveInGetNumDevs")
)

func runPlatform(cfg *config.Config, backend string) Report {
	if backend == "" {
		backend = "windows"
	}
	report := Report{
		Platform: "windows",
		Backend:  backend,
		Info: []string{
			"配置文件：" + configDisplayPath(),
		},
	}
	report.Checks = append(report.Checks,
		Check{Name: "全局热键", OK: true, Severity: Required, Detail: "Windows 全局按键监听可用"},
		windowsMicrophoneCheck(cfg),
		windowsRecordingBackendCheck(),
		Check{Name: "剪贴板与自动上屏", OK: true, Severity: Required, Detail: "Windows 原生 API 可用"},
		secretEncryptionCheck(cfg),
	)
	if cfg.Voice.Enabled {
		report.Checks = append(report.Checks, asrConfigCheck(cfg))
	}
	return report
}

func windowsMicrophoneCheck(cfg *config.Config) Check {
	count, _, _ := doctorProcWaveInGetNumDevs.Call()
	if count == 0 {
		return Check{
			Name: "麦克风录音", OK: false, Severity: Required,
			Detail: "没有检测到录音设备",
			Fix:    "连接或启用麦克风，并在 Windows 设置 → 隐私和安全性 → 麦克风中允许桌面应用访问。",
		}
	}
	detail := fmt.Sprintf("检测到 %d 个输入设备", count)
	if device := strings.TrimSpace(cfg.Voice.Device); device != "" {
		detail += "；配置设备=" + device
	}
	return Check{
		Name: "麦克风录音", OK: true, Severity: Required, Detail: detail,
		Notes: []string{"首次录音失败时，请检查 Windows 麦克风隐私权限。"},
	}
}

// windowsRecordingBackendCheck checks that at least one recording backend (ffmpeg or sox) is available.
func windowsRecordingBackendCheck() Check {
	backends := []string{"ffmpeg", "sox"}
	return commandAnyCheck("录音后端", Required, backends,
		"安装 ffmpeg（推荐）或 sox，并确保在 PATH 中。",
	)
}