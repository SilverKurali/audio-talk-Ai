package doctor

import (
	"bytes"
	"strings"
	"testing"

	"gitee.com/AY77-OP/audio-talk-ai/config"
)

func TestPlatformName(t *testing.T) {
	cases := map[string]string{
		"darwin":  "macOS",
		"Darwin":  "macOS",
		"linux":   "Linux",
		"windows": "Windows",
		"freebsd": "freebsd", // unknown -> returned as-is
		"":        "",
	}
	for in, want := range cases {
		if got := platformName(in); got != want {
			t.Errorf("platformName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFallback(t *testing.T) {
	if got := fallback("value", "def"); got != "value" {
		t.Errorf("fallback(value) = %q, want value", got)
	}
	if got := fallback("   ", "def"); got != "def" {
		t.Errorf("fallback(blank) = %q, want def", got)
	}
	if got := fallback("", "def"); got != "def" {
		t.Errorf("fallback(empty) = %q, want def", got)
	}
}

func TestReportHealthy(t *testing.T) {
	t.Run("all OK -> healthy", func(t *testing.T) {
		r := Report{Checks: []Check{
			{Name: "a", OK: true, Severity: Required},
			{Name: "b", OK: true, Severity: Warning},
		}}
		if !r.Healthy() {
			t.Fatal("expected healthy")
		}
	})

	t.Run("required failure -> not healthy", func(t *testing.T) {
		r := Report{Checks: []Check{
			{Name: "a", OK: false, Severity: Required},
		}}
		if r.Healthy() {
			t.Fatal("required failure should be unhealthy")
		}
	})

	t.Run("warning failure alone stays healthy", func(t *testing.T) {
		r := Report{Checks: []Check{
			{Name: "a", OK: false, Severity: Warning},
		}}
		if !r.Healthy() {
			t.Fatal("warning-only failure should stay healthy")
		}
	})
}

func TestReportPrint(t *testing.T) {
	r := Report{
		Platform: "linux",
		Backend:  "wayland",
		Info:     []string{"info line 1"},
		Checks: []Check{
			{Name: "good", OK: true, Severity: Required},
			{Name: "warn", OK: false, Severity: Warning, Fix: "fix-warn"},
			{Name: "bad", OK: false, Severity: Required, Fix: "fix-bad"},
		},
	}
	var buf bytes.Buffer
	r.Print(&buf)
	out := buf.String()

	for _, want := range []string{"Audio Talk AI", "Linux", "wayland", "✓ good", "! warn", "✗ bad", "处理：fix-bad", "需要处理"} {
		if !strings.Contains(out, want) {
			t.Errorf("Print output missing %q:\n%s", want, out)
		}
	}
	// a healthy report prints the OK banner
	var buf2 bytes.Buffer
	(Report{Checks: []Check{{Name: "ok", OK: true, Severity: Required}}}).Print(&buf2)
	if !strings.Contains(buf2.String(), "环境正常") {
		t.Errorf("healthy banner missing:\n%s", buf2.String())
	}
}

func TestASRConfigCheck(t *testing.T) {
	t.Run("no providers -> warning", func(t *testing.T) {
		c := asrConfigCheck(&config.Config{})
		if c.OK {
			t.Fatal("expected OK=false with no providers")
		}
		if c.Severity != Warning {
			t.Errorf("Severity = %v, want Warning", c.Severity)
		}
	})

	complete := []struct {
		name   string
		provider config.ASRProviderConfig
	}{
		{"doubao", config.ASRProviderConfig{Name: "d", Type: "doubao", AppKey: "k", AccessKey: "s"}},
		{"openai-whisper", config.ASRProviderConfig{Name: "w", Type: "openai-whisper", ApiKey: "k"}},
		{"openai-realtime", config.ASRProviderConfig{Name: "r", Type: "openai-realtime", ApiKey: "k"}},
		{"xiaomi", config.ASRProviderConfig{Name: "m", Type: "xiaomi-mimo-asr", ApiKey: "k"}},
		{"xfyun-spark", config.ASRProviderConfig{Name: "s", Type: "xfyun-spark", AppID: "a", ApiKey: "k", ApiSecret: "sec"}},
	}
	for _, tc := range complete {
		t.Run("complete/"+tc.name, func(t *testing.T) {
			c := asrConfigCheck(&config.Config{ASRs: []config.ASRProviderConfig{tc.provider}})
			if !c.OK {
				t.Errorf("provider %s reported incomplete: %+v", tc.name, c)
			}
		})
	}

	missing := []struct {
		name     string
		provider config.ASRProviderConfig
		wantSub  string
	}{
		{"doubao-missing-access", config.ASRProviderConfig{Name: "d", Type: "doubao", AppKey: "k"}, "access_key"},
		{"whisper-missing-key", config.ASRProviderConfig{Name: "w", Type: "openai-whisper"}, "api_key"},
		{"spark-missing-secret", config.ASRProviderConfig{Name: "s", Type: "xfyun-spark", AppID: "a", ApiKey: "k"}, "api_secret"},
	}
	for _, tc := range missing {
		t.Run("missing/"+tc.name, func(t *testing.T) {
			c := asrConfigCheck(&config.Config{ASRs: []config.ASRProviderConfig{tc.provider}})
			if c.OK {
				t.Fatal("expected OK=false")
			}
			if !strings.Contains(c.Detail, tc.wantSub) {
				t.Errorf("Detail %q does not mention %q", c.Detail, tc.wantSub)
			}
		})
	}
}
