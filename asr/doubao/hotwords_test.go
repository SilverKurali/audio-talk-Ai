package doubao

import (
	"encoding/json"
	"testing"
)

func TestHotwordsContextDedup(t *testing.T) {
	got, err := hotwordsContext([]string{"你好", "世界", "你好", "  ", "世界"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		Hotwords []map[string]string `json:"hotwords"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON %q: %v", got, err)
	}
	// dedup (你好, 世界) + blank dropped, order preserved
	want := []string{"你好", "世界"}
	if len(parsed.Hotwords) != len(want) {
		t.Fatalf("got %d hotwords, want %d: %v", len(parsed.Hotwords), len(want), parsed.Hotwords)
	}
	for i, w := range want {
		if parsed.Hotwords[i]["word"] != w {
			t.Errorf("hotword[%d] = %q, want %q (order not preserved?)", i, parsed.Hotwords[i]["word"], w)
		}
	}
}

func TestHotwordsContextEmpty(t *testing.T) {
	cases := [][]string{
		nil,
		{},
		{"", "   ", "\t"},
	}
	for i, words := range cases {
		got, err := hotwordsContext(words)
		if err != nil {
			t.Fatalf("case %d unexpected error: %v", i, err)
		}
		if got != "" {
			t.Fatalf("case %d hotwordsContext(%v) = %q, want \"\"", i, words, got)
		}
	}
}

func TestHotwordsContextTrims(t *testing.T) {
	got, err := hotwordsContext([]string{"  hello  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		Hotwords []map[string]string `json:"hotwords"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Hotwords) != 1 || parsed.Hotwords[0]["word"] != "hello" {
		t.Fatalf("word not trimmed: %v", parsed.Hotwords)
	}
}
