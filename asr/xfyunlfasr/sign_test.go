package xfyunlfasr

import (
	"encoding/json"
	"testing"
)

// ---- little-endian writers ----

func TestPutLE16(t *testing.T) {
	b := make([]byte, 2)
	putLE16(b, 0x0102)
	want := []byte{0x02, 0x01}
	if b[0] != want[0] || b[1] != want[1] {
		t.Fatalf("putLE16 = % x, want % x", b, want)
	}
}

func TestPutLE32(t *testing.T) {
	b := make([]byte, 4)
	putLE32(b, 0x01020304)
	want := []byte{0x04, 0x03, 0x02, 0x01}
	for i := range want {
		if b[i] != want[i] {
			t.Fatalf("putLE32 = % x, want % x", b, want)
		}
	}
}

// ---- pcmToWAV header ----

func TestPCMToWAVHeader(t *testing.T) {
	t.Run("empty pcm still gets a 44-byte header", func(t *testing.T) {
		out := pcmToWAV(nil, 16000, 1)
		if len(out) != 44 {
			t.Fatalf("len = %d, want 44", len(out))
		}
		assertWAVHeader(t, out, 16000, 1, 0)
	})

	t.Run("pcm payload appended verbatim", func(t *testing.T) {
		pcm := []byte{0x01, 0x02, 0x03, 0x04}
		out := pcmToWAV(pcm, 16000, 1)
		if len(out) != 44+len(pcm) {
			t.Fatalf("len = %d, want %d", len(out), 44+len(pcm))
		}
		// payload bytes unchanged after header
		for i, v := range pcm {
			if out[44+i] != v {
				t.Fatalf("payload byte %d = %x, want %x", i, out[44+i], v)
			}
		}
		assertWAVHeader(t, out, 16000, 1, len(pcm))
	})

	t.Run("stereo doubles block align / byte rate", func(t *testing.T) {
		out := pcmToWAV(nil, 8000, 2)
		// byteRate = sampleRate*channels*2 = 8000*2*2 = 32000
		if le32(out[28:32]) != 32000 {
			t.Fatalf("byteRate = %d, want 32000", le32(out[28:32]))
		}
		// blockAlign = channels*2 = 4
		if le16(out[32:34]) != 4 {
			t.Fatalf("blockAlign = %d, want 4", le16(out[32:34]))
		}
	})
}

func assertWAVHeader(t *testing.T, out []byte, sampleRate, channels, dataSize int) {
	t.Helper()
	chunks := [][2]int{{0, 4}, {8, 12}, {12, 16}, {36, 40}}
	want := []string{"RIFF", "WAVE", "fmt ", "data"}
	for i, c := range chunks {
		if string(out[c[0]:c[1]]) != want[i] {
			t.Errorf("tag at [%d:%d] = %q, want %q", c[0], c[1], out[c[0]:c[1]], want[i])
		}
	}
	if le16(out[22:24]) != uint16(channels) {
		t.Errorf("channels = %d, want %d", le16(out[22:24]), channels)
	}
	if le32(out[24:28]) != uint32(sampleRate) {
		t.Errorf("sampleRate = %d, want %d", le32(out[24:28]), sampleRate)
	}
	if le16(out[34:36]) != 16 {
		t.Errorf("bitsPerSample = %d, want 16", le16(out[34:36]))
	}
	if le32(out[4:8]) != uint32(36+dataSize) {
		t.Errorf("RIFF size = %d, want %d", le32(out[4:8]), 36+dataSize)
	}
	if le32(out[40:44]) != uint32(dataSize) {
		t.Errorf("data size = %d, want %d", le32(out[40:44]), dataSize)
	}
}

func le16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// ---- sortStrings ----

func TestSortStrings(t *testing.T) {
	cases := [][]string{
		{"banana", "apple", "cherry"},
		{"c", "b", "a"},
		{},
		{"single"},
		{"z", "y", "x", "w", "v", "u"},
	}
	for i, in := range cases {
		got := append([]string(nil), in...)
		sortStrings(got)
		for j := 1; j < len(got); j++ {
			if got[j-1] > got[j] {
				t.Fatalf("case %d not sorted: %v", i, got)
			}
		}
	}
}

// ---- encodeParams ----

func TestEncodeParams(t *testing.T) {
	got := encodeParams(map[string]string{"b": "2", "a": "1"})
	if got != "a=1&b=2" {
		t.Fatalf("encodeParams = %q, want %q", got, "a=1&b=2")
	}
	if encodeParams(map[string]string{}) != "" {
		t.Fatalf("empty encodeParams should be \"\"")
	}
	// special chars get percent-encoded (url.Values.Encode semantics)
	got = encodeParams(map[string]string{"k": "a b+c"})
	if got != "k=a+b%2Bc" {
		t.Fatalf("encodeParams escape = %q, want %q", got, "k=a+b%2Bc")
	}
}

// ---- standardSigna (golden vector, cross-checked with Python hmac/sha1) ----

func TestStandardSigna(t *testing.T) {
	c := &client{appID: "test_app", apiSecret: "test_secret"}
	// expected independently computed:
	//   md5hex("test_app"+"1700000000000") -> HMAC-SHA1("test_secret") -> base64
	const want = "eSg8Bi4MLhy6YPwXeQNqX/SMHTM="
	if got := c.standardSigna("1700000000000"); got != want {
		t.Fatalf("standardSigna = %q, want %q", got, want)
	}
	// different ts -> different signature (not a constant)
	if c.standardSigna("1700000000001") == want {
		t.Fatal("standardSigna identical for different ts")
	}
}

// ---- largeModelSignature (golden vectors) ----

func TestLargeModelSignature(t *testing.T) {
	c := &client{apiSecret: "lm_secret"}

	t.Run("clean alnum values", func(t *testing.T) {
		params := map[string]string{
			"accessKeyId":     "AKvalue",
			"appId":           "AID",
			"dateTime":        "20260812T120000",
			"language":        "cn",
			"signatureRandom": "RND",
		}
		const want = "jfE+yj2EZraYk5kFbqadAQeLQz8="
		if got := c.largeModelSignature(params); got != want {
			t.Fatalf("largeModelSignature = %q, want %q", got, want)
		}
	})

	t.Run("value url-encoded (+ -> %2B, space -> +)", func(t *testing.T) {
		// Go url.QueryEscape on "2026+08 12" yields "2026%2B08+12"
		params := map[string]string{
			"appId":    "A",
			"dateTime": "2026+08 12",
		}
		const want = "S8cvjFsMKxZ9y9qem9khyu+2uz8="
		if got := c.largeModelSignature(params); got != want {
			t.Fatalf("largeModelSignature(escape) = %q, want %q", got, want)
		}
	})

	t.Run("signature key excluded from base string", func(t *testing.T) {
		params := map[string]string{"appId": "A", "signature": "ignored"}
		got := c.largeModelSignature(params)
		// signature key must not influence the result
		params2 := map[string]string{"appId": "A"}
		if got != c.largeModelSignature(params2) {
			t.Fatalf("signature key leaked into base string: %q vs %q", got, c.largeModelSignature(params2))
		}
	})
}

// ---- lattice parsing ----

// makeLattice builds the nested JSON lattice used by parseLatticeResult/parseFastResult.
func makeLattice(t *testing.T, words ...string) string {
	t.Helper()
	inner, err := json.Marshal(map[string]interface{}{
		"st": map[string]interface{}{
			"rt": []map[string]interface{}{
				{"ws": []map[string]interface{}{
					{"cw": []map[string]string{
						{"w": words[0]},
					}},
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := json.Marshal(map[string]interface{}{
		"lattice": []map[string]string{
			{"json_1best": string(inner)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(outer)
}

func TestParseLatticeResult(t *testing.T) {
	c := &client{}

	t.Run("empty string returns empty", func(t *testing.T) {
		got, err := c.parseLatticeResult("")
		if err != nil || got != "" {
			t.Fatalf("parseLatticeResult(\"\") = (%q,%v), want (\"\",nil)", got, err)
		}
	})

	t.Run("valid lattice concatenates cw words", func(t *testing.T) {
		in := makeLattice(t, "你好")
		got, err := c.parseLatticeResult(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "你好" {
			t.Fatalf("parseLatticeResult = %q, want %q", got, "你好")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := c.parseLatticeResult("{not json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestParseFastResult(t *testing.T) {
	c := &client{}

	t.Run("valid lattice", func(t *testing.T) {
		in := makeLattice(t, "极速")
		got, err := c.parseFastResult(in)
		if err != nil || got != "极速" {
			t.Fatalf("parseFastResult = (%q,%v), want (极速,nil)", got, err)
		}
	})

	t.Run("invalid JSON falls back to raw string", func(t *testing.T) {
		raw := "plain text result"
		got, err := c.parseFastResult(raw)
		if err != nil || got != raw {
			t.Fatalf("parseFastResult(invalid) = (%q,%v), want (%q,nil)", got, err, raw)
		}
	})
}
