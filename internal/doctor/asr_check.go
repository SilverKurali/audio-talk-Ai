package doctor

import (
	"fmt"
	"strings"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

func asrConfigCheck(cfg *config.Config) Check {
	providers := cfg.ResolveASRProviders()
	if len(providers) == 0 {
		return Check{
			Name: "ASR 配置", OK: false, Severity: Warning,
			Detail: "未配置 ASR 提供商",
			Fix:    "在 config.toml 中配置 [[asr_providers]] 或在 [voice] 中填写 app_key 和 access_key。",
		}
	}
	for _, p := range providers {
		var missing []string
		switch p.Type {
		case "doubao":
			if strings.TrimSpace(p.AppKey) == "" {
				missing = append(missing, "app_key")
			}
			if strings.TrimSpace(p.AccessKey) == "" {
				missing = append(missing, "access_key")
			}
		case "openai-realtime", "openai-whisper", "mimo-asr":
			if strings.TrimSpace(p.ApiKey) == "" {
				missing = append(missing, "api_key")
			}
		case "xfyun-spark":
			if strings.TrimSpace(p.AppID) == "" {
				missing = append(missing, "app_id")
			}
			if strings.TrimSpace(p.ApiKey) == "" {
				missing = append(missing, "api_key")
			}
			if strings.TrimSpace(p.ApiSecret) == "" {
				missing = append(missing, "api_secret")
			}
		}
		if len(missing) > 0 {
			return Check{
				Name: "ASR 配置", OK: false, Severity: Warning,
				Detail: fmt.Sprintf("提供商 %q 缺少 %s", p.Name, strings.Join(missing, ", ")),
				Fix:    fmt.Sprintf("在 [[asr_providers]] name=%q 中填写 %s。", p.Name, strings.Join(missing, ", ")),
			}
		}
	}
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name
	}
	return Check{Name: "ASR 配置", OK: true, Severity: Warning, Detail: fmt.Sprintf("%d 个提供商: %s", len(providers), strings.Join(names, ", "))}
}
