package xfyunspark

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := []struct{ in, want string }{
		{"zh-CN", "zh_cn"},
		{"zh", "zh_cn"},
		{"ZH", "zh_cn"},
		{"en-US", "en_us"},
		{"English", "en_us"},
		{"fr", "fr_fr"},
		{"ja", "ja_ja"},
		{"", "zh_cn"},   // empty falls back to zh_cn
		{"x", "zh_cn"}, // too short falls back to zh_cn
	}
	for _, tc := range cases {
		if got := normalizeLang(tc.in); got != tc.want {
			t.Errorf("normalizeLang(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// decodeResultText base64-decodes then extracts {"ws":[{"cw":[{"w":...}]}]} words.

func latticeBase64(t *testing.T, words []string) string {
	t.Helper()
	inner, err := json.Marshal(map[string]interface{}{
		"ws": []map[string]interface{}{
			{"cw": cwList(words)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(inner)
}

func cwList(words []string) []map[string]string {
	out := make([]map[string]string, len(words))
	for i, w := range words {
		out[i] = map[string]string{"w": w}
	}
	return out
}

func TestDecodeResultText(t *testing.T) {
	t.Run("concatenates words", func(t *testing.T) {
		enc := latticeBase64(t, []string{"你好", "世界"})
		got := (&client{}).decodeResultText(enc)
		if got != "你好世界" {
			t.Fatalf("decodeResultText = %q, want %q", got, "你好世界")
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		if got := (&client{}).decodeResultText(""); got != "" {
			t.Fatalf("decodeResultText(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("invalid base64 returns empty", func(t *testing.T) {
		if got := (&client{}).decodeResultText("!!!not base64!!!"); got != "" {
			t.Fatalf("decodeResultText(invalid b64) = %q, want \"\"", got)
		}
	})

	t.Run("invalid JSON returns empty", func(t *testing.T) {
		enc := base64.StdEncoding.EncodeToString([]byte("{not json"))
		if got := (&client{}).decodeResultText(enc); got != "" {
			t.Fatalf("decodeResultText(invalid json) = %q, want \"\"", got)
		}
	})

	t.Run("empty lattice returns empty", func(t *testing.T) {
		enc := latticeBase64(t, nil)
		if got := (&client{}).decodeResultText(enc); got != "" {
			t.Fatalf("decodeResultText(empty lattice) = %q, want \"\"", got)
		}
	})
}
