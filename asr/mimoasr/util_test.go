package mimoasr

import "testing"

func TestNormalizeLang(t *testing.T) {
	cases := []struct{ in, want string }{
		{"zh-CN", "zh"},
		{"zh", "zh"},
		{"en-US", "en"},
		{"English", "en"},
		{"fr", "auto"},  // unsupported -> auto
		{"ja", "auto"},
		{"", "auto"},
		{"  EN  ", "en"},
	}
	for _, tc := range cases {
		if got := normalizeLang(tc.in); got != tc.want {
			t.Errorf("normalizeLang(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	t.Run("shorter than n returned as-is", func(t *testing.T) {
		if got := truncate("abc", 10); got != "abc" {
			t.Fatalf("truncate = %q, want %q", got, "abc")
		}
		if got := truncate("abc", 3); got != "abc" { // equal length
			t.Fatalf("truncate = %q, want %q", got, "abc")
		}
	})

	t.Run("longer truncated with ellipsis", func(t *testing.T) {
		if got := truncate("abcdef", 3); got != "abc..." {
			t.Fatalf("truncate = %q, want %q", got, "abc...")
		}
	})

	t.Run("byte-based (ASCII)", func(t *testing.T) {
		// truncate is byte-based, not rune-based; ASCII is unaffected.
		if got := truncate("hello world", 5); got != "hello..." {
			t.Fatalf("truncate = %q, want %q", got, "hello...")
		}
	})
}
