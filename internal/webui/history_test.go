package webui

import (
	"testing"
	"time"
)

func TestHistoryAddAutoFills(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	h.Add(HistoryEntry{Text: "你好世界", Provider: "doubao", Duration: 1.5})

	if len(h.items) != 1 {
		t.Fatalf("items = %d, want 1", len(h.items))
	}
	e := h.items[0]
	if e.ID != 1 {
		t.Errorf("ID = %d, want 1", e.ID)
	}
	if e.Chars != 4 { // len([]rune("你好世界"))
		t.Errorf("Chars = %d, want 4", e.Chars)
	}
	if e.CreatedAt.IsZero() {
		t.Error("CreatedAt should be auto-filled when zero")
	}
	// second add increments ID
	h.Add(HistoryEntry{Text: "x"})
	if h.items[1].ID != 2 {
		t.Errorf("second ID = %d, want 2", h.items[1].ID)
	}
}

func TestHistoryListNewestFirst(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		h.Add(HistoryEntry{Text: "t", CreatedAt: base.Add(time.Duration(i) * time.Hour)})
	}
	items, total := h.List(0, 10)
	if total != 5 || len(items) != 5 {
		t.Fatalf("List total=%d len=%d, want 5/5", total, len(items))
	}
	// newest first: last added (base+4h) comes first
	if !items[0].CreatedAt.Equal(base.Add(4 * time.Hour)) {
		t.Errorf("first item not newest: %v", items[0].CreatedAt)
	}

	t.Run("offset+limit window", func(t *testing.T) {
		items, _ := h.List(2, 2) // skip 2 newest, take 2
		if len(items) != 2 {
			t.Fatalf("len = %d, want 2", len(items))
		}
	})

	t.Run("offset beyond total returns nil", func(t *testing.T) {
		items, total := h.List(100, 10)
		if items != nil || total != 5 {
			t.Fatalf("List(100,10) = (%v,%d), want (nil,5)", items, total)
		}
	})
}

func TestHistoryStats(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	h.Add(HistoryEntry{Text: "ab", Duration: 1.0})
	h.Add(HistoryEntry{Text: "cde", Duration: 2.5})
	sessions, chars, dur := h.Stats()
	if sessions != 2 || chars != 5 || dur != 3.5 {
		t.Fatalf("Stats = (%d,%d,%v), want (2,5,3.5)", sessions, chars, dur)
	}
}

func TestHistoryDeleteByIDs(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	for i := 0; i < 4; i++ {
		h.Add(HistoryEntry{Text: "x"})
	}
	deleted := h.DeleteByIDs([]int{2, 3})
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if len(h.items) != 2 {
		t.Fatalf("remaining = %d, want 2", len(h.items))
	}
	// ids 1 and 4 remain
	ids := map[int]bool{}
	for _, e := range h.items {
		ids[e.ID] = true
	}
	if !ids[1] || !ids[4] || ids[2] || ids[3] {
		t.Errorf("wrong survivors: %v", ids)
	}
	// idempotent: deleting again removes nothing
	if h.DeleteByIDs([]int{2, 3}) != 0 {
		t.Error("re-delete should remove 0")
	}
}

func TestHistoryDeleteByDateRange(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	d1 := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	h.Add(HistoryEntry{Text: "a", CreatedAt: d1})
	h.Add(HistoryEntry{Text: "b", CreatedAt: d2})
	h.Add(HistoryEntry{Text: "c", CreatedAt: d3})

	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 18, 23, 59, 59, 0, time.UTC)
	deleted := h.DeleteByDateRange(from, to)
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only d2 in range)", deleted)
	}
	if len(h.items) != 2 {
		t.Fatalf("remaining = %d, want 2", len(h.items))
	}
}

func TestHistoryExportAllNewestFirst(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h.Add(HistoryEntry{Text: "old", CreatedAt: base})
	h.Add(HistoryEntry{Text: "new", CreatedAt: base.Add(time.Hour)})
	all := h.ExportAll()
	if len(all) != 2 || all[0].Text != "new" || all[1].Text != "old" {
		t.Fatalf("ExportAll order wrong: %+v", all)
	}
}

func TestHistoryDateStats(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	day := func(s string) time.Time {
		tt, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return tt
	}
	h.Add(HistoryEntry{Text: "a", CreatedAt: day("2026-01-05")})
	h.Add(HistoryEntry{Text: "b", CreatedAt: day("2026-01-05")}) // same day dedup
	h.Add(HistoryEntry{Text: "c", CreatedAt: day("2026-01-03")})
	h.Add(HistoryEntry{Text: "d", CreatedAt: day("2026-02-01")})

	stats := h.DateStats()
	jan := stats["2026"]["01"]
	if len(jan) != 2 {
		t.Fatalf("Jan days = %v, want [3 5]", jan)
	}
	// days sorted ascending, deduped
	if jan[0] != 3 || jan[1] != 5 {
		t.Errorf("Jan days = %v, want [3 5]", jan)
	}
	if len(stats["2026"]["02"]) != 1 || stats["2026"]["02"][0] != 1 {
		t.Errorf("Feb days = %v, want [1]", stats["2026"]["02"])
	}
}

func TestHistoryTrimIfExceeds(t *testing.T) {
	h := NewHistoryStore(t.TempDir())
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		h.Add(HistoryEntry{Text: "x", CreatedAt: base.Add(time.Duration(i) * time.Hour)})
	}
	removed := h.TrimIfExceeds(3)
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if len(h.items) != 3 {
		t.Fatalf("remaining = %d, want 3", len(h.items))
	}
	// oldest dropped: remaining are the 3 newest (base+2h..+4h)
	if !h.items[0].CreatedAt.Equal(base.Add(2 * time.Hour)) {
		t.Errorf("TrimIfExceeds dropped wrong end: first = %v", h.items[0].CreatedAt)
	}
	// already under max -> no-op
	if h.TrimIfExceeds(10) != 0 {
		t.Error("trim under max should be no-op")
	}
}

func TestHistoryPersistsAcrossStores(t *testing.T) {
	dir := t.TempDir()
	h1 := NewHistoryStore(dir)
	h1.Add(HistoryEntry{Text: "persist me"})
	h1.Add(HistoryEntry{Text: "second"})

	// a new store pointing at the same dir must load the saved items + continue IDs
	h2 := NewHistoryStore(dir)
	if len(h2.items) != 2 {
		t.Fatalf("loaded %d items, want 2", len(h2.items))
	}
	h2.Add(HistoryEntry{Text: "third"})
	if h2.items[2].ID != 3 {
		t.Errorf("next ID after reload = %d, want 3", h2.items[2].ID)
	}
}
