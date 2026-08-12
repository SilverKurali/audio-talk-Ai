package openaiwhisper

import "testing"

func TestNormalizeLang(t *testing.T) {
	cases := []struct{ in, want string }{
		{"zh-CN", "zh"},
		{"zh", "zh"},
		{"en-US", "en"},
		{"English", "en"},
		{"fr", "fr"},
		{"ja", "ja"},
		{"x", "x"},   // too short -> as-is (lowercased)
		{"", ""},     // empty -> empty
		{"  FR  ", "fr"},
	}
	for _, tc := range cases {
		if got := normalizeLang(tc.in); got != tc.want {
			t.Errorf("normalizeLang(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPCMToWAVHeader(t *testing.T) {
	out := pcmToWAV(nil, 16000, 1)
	if len(out) != 44 {
		t.Fatalf("len = %d, want 44", len(out))
	}
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WAVE" {
		t.Fatalf("missing RIFF/WAVE markers: % x", out[:12])
	}
	if string(out[36:40]) != "data" {
		t.Fatalf("missing data chunk marker: % x", out[36:40])
	}
	// sampleRate 16000 little-endian at offset 24
	if uint32(out[24])|uint32(out[25])<<8|uint32(out[26])<<16|uint32(out[27])<<24 != 16000 {
		t.Fatalf("sampleRate not 16000: % x", out[24:28])
	}
	// payload appended
	pcm := []byte{0x01, 0x02, 0x03}
	out2 := pcmToWAV(pcm, 16000, 1)
	if len(out2) != 44+len(pcm) {
		t.Fatalf("len = %d, want %d", len(out2), 44+len(pcm))
	}
}
