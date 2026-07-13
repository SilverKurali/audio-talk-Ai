//go:build linux

package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

func runPlatform(cfg *config.Config, backend string) Report {
	backend = detectBackend(backend)
	report := Report{Platform: "linux", Backend: backend}
	if desktop := desktopInfo(); desktop != "" {
		report.Info = append(report.Info, "桌面环境："+desktop)
	}

	switch backend {
	case "wayland":
		report.Checks = append(report.Checks, waylandChecks(cfg)...)
	case "x11":
		report.Checks = append(report.Checks, x11Checks(cfg)...)
	default:
		report.Checks = append(report.Checks, Check{
			Name: "热键后端", OK: false, Severity: Required,
			Detail: fmt.Sprintf("unknown backend %q", backend),
			Fix:    "使用 --backend wayland 或 --backend x11，或设置 JUST_TALK_BACKEND=wayland/x11。",
		})
	}

	report.Checks = append(report.Checks, platformDependencyChecks(backend)...)
	report.Checks = append(report.Checks, secretEncryptionCheck(cfg))
	if cfg.Voice.Enabled {
		report.Checks = append(report.Checks, asrConfigCheck(cfg))
	}
	return report
}

func detectBackend(backend string) string {
	if backend != "" {
		return backend
	}
	if env := os.Getenv("JUST_TALK_BACKEND"); env != "" {
		return env
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		return "wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	return "unknown"
}

func recordingBackendCheck() Check {
	return commandAnyCheck(
		"录音后端",
		Required,
		[]string{"arecord"},
		"安装 alsa-utils，确保 arecord 可用。",
	)
}

func waylandChecks(cfg *config.Config) []Check {
	checks := []Check{
		{
			Name: "Wayland 会话", OK: os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland",
			Severity: Required,
			Detail:   envDetail("WAYLAND_DISPLAY", "XDG_SESSION_TYPE"),
			Fix:      "在 Wayland/Sway 会话里运行，或用 --backend x11 切换到 X11。",
		},
	}

	checks = append(checks, inputAccessCheck())
	return checks
}

func x11Checks(cfg *config.Config) []Check {
	return []Check{
		{
			Name: "X11 会话", OK: os.Getenv("DISPLAY") != "",
			Severity: Required,
			Detail:   envDetail("DISPLAY"),
			Fix:      "在 X11 会话里运行，或用 --backend wayland 切换到 Wayland。",
		},
	}
}

func platformDependencyChecks(backend string) []Check {
	checks := []Check{
		recordingBackendCheck(),
	}

	switch backend {
	case "wayland":
		checks = append(checks,
			commandAllCheck("Wayland 剪贴板", Required, []string{"wl-copy", "wl-paste"}, "安装 wl-clipboard。"),
			waylandAutotypeCheck(),
		)
	case "x11":
		checks = append(checks,
			commandAllCheck("X11 剪贴板", Required, []string{"xclip"}, "安装 xclip。"),
		)
	}
	return checks
}

func waylandAutotypeCheck() Check {
	isKDE := isKDEPlasma()
	if path, err := exec.LookPath("wtype"); err == nil {
		check := Check{Name: "Wayland 自动上屏", OK: true, Severity: Required, Detail: "wtype=" + path}
		if isKDE {
			check.Notes = []string{"检测到 KDE Plasma；如果 wtype 不可用或无效，请改用 /dev/uinput 权限。"}
		}
		return check
	}
	if f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0); err == nil {
		_ = f.Close()
		return Check{Name: "Wayland 自动上屏", OK: true, Severity: Required, Detail: "/dev/uinput 可写"}
	}
	fix := "安装 wtype，或通过 udev 规则/用户组授予当前用户写 /dev/uinput 的权限。"
	if isKDE {
		fix = uinputPermissionFix()
	}
	return Check{
		Name:     "Wayland 自动上屏",
		OK:       false,
		Severity: Required,
		Detail:   "wtype 不可用，且当前用户不能写 /dev/uinput",
		Fix:      fix,
	}
}

func uinputPermissionFix() string {
	return "KDE Plasma Wayland 建议使用 /dev/uinput。执行：sudo groupadd -f uinput；sudo usermod -aG uinput $USER；写入 /etc/udev/rules.d/70-uinput.rules：KERNEL==\"uinput\", MODE=\"0660\", GROUP=\"uinput\", OPTIONS+=\"static_node=uinput\"；然后 sudo modprobe uinput && sudo udevadm control --reload-rules && sudo udevadm trigger /dev/uinput，最后重新登录。"
}

func desktopInfo() string {
	var parts []string
	for _, name := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "KDE_SESSION_VERSION"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			parts = append(parts, name+"="+v)
		}
	}
	return strings.Join(parts, " / ")
}

func isKDEPlasma() bool {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + " " + os.Getenv("DESKTOP_SESSION"))
	return strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma") || os.Getenv("KDE_SESSION_VERSION") != ""
}

func inputAccessCheck() Check {
	events, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(events) == 0 {
		return Check{Name: "输入设备权限", OK: false, Severity: Required, Detail: "没有找到 /dev/input/event*", Fix: "确认系统存在 evdev 输入设备。"}
	}
	for _, dev := range events {
		f, err := os.Open(dev)
		if err == nil {
			_ = f.Close()
			return Check{Name: "输入设备权限", OK: true, Severity: Required, Detail: dev + " 可读"}
		}
	}

	groupDetail := currentGroupDetail()
	return Check{
		Name:     "输入设备权限",
		OK:       false,
		Severity: Required,
		Detail:   "无法读取 /dev/input/event*；" + groupDetail,
		Fix:      "把当前用户加入 input 组：sudo usermod -aG input $USER，然后重新登录。",
	}
}

func envDetail(names ...string) string {
	var parts []string
	for _, name := range names {
		value := os.Getenv(name)
		if value == "" {
			value = "<empty>"
		}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, " / ")
}

func currentGroupDetail() string {
	u, err := user.Current()
	if err != nil {
		return "无法读取当前用户组"
	}
	gids, err := u.GroupIds()
	if err != nil {
		return "无法读取当前用户组"
	}
	for _, gid := range gids {
		g, err := user.LookupGroupId(gid)
		if err == nil && g.Name == "input" {
			return "当前用户已在 input 组，但当前会话可能尚未刷新"
		}
	}
	input, err := user.LookupGroup("input")
	if err != nil {
		return "系统没有 input 组"
	}
	if _, err := strconv.Atoi(input.Gid); err != nil {
		return "input 组存在"
	}
	return "当前用户不在 input 组"
}
