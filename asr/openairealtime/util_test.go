package openairealtime

import (
	"bytes"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := []struct{ in, want string }{
		{"zh-CN", "zh"},
		{"ZH", "zh"},
		{"en-US", "en"},
		{"ja-JP", "ja"},
		{"fr", "fr"},
		{"x", "x"},  // too short -> returned as-is (lowercased)
		{"", ""},    // empty -> empty
		{"  EN  ", "en"}, // trimmed + lowercased
	}
	for _, tc := range cases {
		if got := normalizeLang(tc.in); got != tc.want {
			t.Errorf("normalizeLang(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResample16to24(t *testing.T) {
	t.Run("empty input returned unchanged", func(t *testing.T) {
		in := []byte{}
		if got := resample16to24(in); !bytes.Equal(got, in) {
			t.Fatalf("empty resample = %v, want unchanged %v", got, in)
		}
		// odd-length (1 byte, 0 full samples) also unchanged
		odd := []byte{0xAB}
		if got := resample16to24(odd); !bytes.Equal(got, odd) {
			t.Fatalf("odd resample = %v, want unchanged %v", got, odd)
		}
	})

	t.Run("output length is 3/2 of input samples", func(t *testing.T) {
		// 4 bytes = 2 samples -> 3 samples -> 6 bytes
		in := []byte{0x10, 0x00, 0x20, 0x00}
		out := resample16to24(in)
		if len(out) != 6 {
			t.Fatalf("len = %d, want 6", len(out))
		}
	})

	t.Run("golden vector (cross-checked with Python)", func(t *testing.T) {
		// input samples [16, 32] -> output samples [16, 26, 21]
		in := []byte{0x10, 0x00, 0x20, 0x00}
		want := []byte{16, 0, 26, 0, 21, 0}
		if got := resample16to24(in); !bytes.Equal(got, want) {
			t.Fatalf("resample = %v, want %v", got, want)
		}
	})

	t.Run("all-zero input -> all-zero output", func(t *testing.T) {
		in := make([]byte, 8) // 4 samples of zero
		out := resample16to24(in)
		want := make([]byte, 12) // 4 samples -> 6 out -> 12 bytes, all zero
		if !bytes.Equal(out, want) {
			t.Fatalf("zero resample = %v, want %v", out, want)
		}
	})
}
