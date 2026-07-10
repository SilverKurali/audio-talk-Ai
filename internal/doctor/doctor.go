package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

type Severity int

const (
	Required Severity = iota
	Warning
)

type Check struct {
	Name     string
	OK       bool
	Severity Severity
	Detail   string
	Notes    []string
	Fix      string
}

type Report struct {
	Platform string
	Backend  string
	Info     []string
	Checks   []Check
}

func Run(cfg *config.Config, backend string) Report {
	return runPlatform(cfg, backend)
}

func (r Report) Healthy() bool {
	for _, check := range r.Checks {
		if check.Severity == Required && !check.OK {
			return false
		}
	}
	return true
}

func (r Report) Print(w io.Writer) {
	fmt.Fprintln(w, "Audio Talk AI 环境检查")
	fmt.Fprintf(w, "平台：%s", platformName(fallback(r.Platform, "unknown")))
	if r.Backend != "" {
		fmt.Fprintf(w, " / %s", r.Backend)
	}
	fmt.Fprintln(w)
	for _, line := range r.Info {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintln(w, line)
		}
	}
	fmt.Fprintln(w)

	for _, check := range r.Checks {
		mark := "✓"
		if !check.OK {
			if check.Severity == Warning {
				mark = "!"
			} else {
				mark = "✗"
			}
		}
		fmt.Fprintf(w, "%s %s", mark, check.Name)
		if check.Detail != "" {
			fmt.Fprintf(w, "：%s", check.Detail)
		}
		fmt.Fprintln(w)
		for _, note := range check.Notes {
			if strings.TrimSpace(note) != "" {
				fmt.Fprintf(w, "  %s\n", note)
			}
		}
		if !check.OK && check.Fix != "" {
			fmt.Fprintf(w, "  处理：%s\n", check.Fix)
		}
	}

	if r.Healthy() {
		fmt.Fprintln(w, "\n结果：环境正常")
	} else {
		fmt.Fprintln(w, "\n结果：需要处理上面的项目后再启动 Audio Talk AI。")
	}
}

func fallback(s, v string) string {
	if strings.TrimSpace(s) == "" {
		return v
	}
	return s
}

func platformName(s string) string {
	switch strings.ToLower(s) {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return s
	}
}

// configDisplayPath returns the config file path for display in the report.
func configDisplayPath() string {
	if p := config.FindConfig(); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/audio-talk-ai/config.toml"
	}
	return filepath.Join(home, ".config", "audio-talk-ai", "config.toml")
}

// secretEncryptionCheck reports the at-rest safety of stored API secrets.
// It inspects the on-disk config (via LoadRaw, which does NOT decrypt) so the
// result reflects what is actually written to disk, not the in-memory plaintext.
func secretEncryptionCheck(_ *config.Config) Check {
	raw, err := config.LoadRaw("")
	if err != nil {
		return Check{
			Name:     "密钥加密",
			OK:       false,
			Severity: Warning,
			Detail:   "无法读取配置文件：" + err.Error(),
		}
	}

	if config.HasPlaintextSecrets(raw) {
		return Check{
			Name:     "密钥加密",
			OK:       false,
			Severity: Warning,
			Detail:   "配置文件里仍有明文密钥",
			Notes: []string{
				"密钥以明文形式存在于 " + configDisplayPath() + "，本机其他用户或备份副本可读到。",
			},
			Fix: "正常运行一次本程序即可自动把明文密钥加密为密文（enc: 前缀）。",
		}
	}

	// No plaintext secrets. If encryption is in use, verify the key file.
	if config.HasEncryptedSecrets(raw) {
		keyPath := config.KeyFilePath()
		info, err := os.Stat(keyPath)
		if err != nil {
			return Check{
				Name:     "密钥加密",
				OK:       false,
				Severity: Warning,
				Detail:   "解密钥匙文件缺失",
				Notes: []string{
					"配置中已有加密密钥，但解密钥匙文件 " + keyPath + " 不存在，密钥将无法解密。",
				},
				Fix: "恢复该钥匙文件，或删除配置中的对应服务商后重新填写密钥（会生成新的钥匙）。",
			}
		}
		mode := info.Mode().Perm()
		if mode != os.FileMode(0600) {
			return Check{
				Name:     "密钥加密",
				OK:       false,
				Severity: Warning,
				Detail:   "钥匙文件权限 " + mode.String() + " 过宽",
				Notes: []string{
					"钥匙文件 " + keyPath + " 应仅本人可读，当前可能其他用户也能读取。",
				},
				Fix: "执行 chmod 600 " + keyPath,
			}
		}
		return Check{
			Name:     "密钥加密",
			OK:       true,
			Severity: Warning,
			Detail:   "已加密，钥匙文件权限 0600",
		}
	}

	// No secrets configured at all.
	return Check{
		Name:     "密钥加密",
		OK:       true,
		Severity: Warning,
		Detail:   "未配置密钥（无需加密）",
	}
}
