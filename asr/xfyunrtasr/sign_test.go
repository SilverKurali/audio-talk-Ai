package xfyunrtasr

import "testing"

// generateSignature sorts params (excl "signature"), skips empty values,
// builds QueryEscape(k)=QueryEscape(v) joined by "&", HMAC-SHA1(secretKey), base64.
// Golden vectors cross-checked independently with Python (hmac/hashlib).

func TestGenerateSignatureGolden(t *testing.T) {
	c := &client{secretKey: "rtasr_secret"}
	params := map[string]string{
		"appId": "AID",
		"lang":  "zh",
		"pd":    "tech",
	}
	const want = "ORog2dAGcT4DL8ThBfaXdSbdt4A="
	if got := c.generateSignature(params); got != want {
		t.Fatalf("generateSignature = %q, want %q", got, want)
	}
}

func TestGenerateSignatureEscapesKeysAndValues(t *testing.T) {
	// Go url.QueryEscape: space -> "+", "+" -> "%2B".
	// base string: "a+k=v%2Bl"
	c := &client{secretKey: "rtasr_secret"}
	params := map[string]string{"a k": "v+l"}
	const want = "ipIGnMWM2f0G29N4PonzFBxrTrY="
	if got := c.generateSignature(params); got != want {
		t.Fatalf("generateSignature(escape) = %q, want %q", got, want)
	}
}

func TestGenerateSignatureProperties(t *testing.T) {
	c := &client{secretKey: "sk"}

	t.Run("signature key excluded", func(t *testing.T) {
		a := map[string]string{"k": "v", "signature": "x"}
		b := map[string]string{"k": "v"}
		if c.generateSignature(a) != c.generateSignature(b) {
			t.Fatal("signature key leaked into base string")
		}
	})

	t.Run("empty values skipped", func(t *testing.T) {
		// an empty value must not change the signature
		base := map[string]string{"k": "v"}
		withEmpty := map[string]string{"k": "v", "extra": ""}
		if c.generateSignature(base) != c.generateSignature(withEmpty) {
			t.Fatal("empty value changed signature")
		}
	})

	t.Run("deterministic regardless of map insertion", func(t *testing.T) {
		// run several times; map iteration order varies but output must not.
		first := c.generateSignature(map[string]string{"c": "3", "a": "1", "b": "2"})
		for i := 0; i < 20; i++ {
			if got := c.generateSignature(map[string]string{"c": "3", "a": "1", "b": "2"}); got != first {
				t.Fatalf("non-deterministic signature: %q vs %q", got, first)
			}
		}
	})
}
