package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

type HistoryEntry struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Provider  string    `json:"provider"`
	Duration  float64   `json:"duration_sec"`
	Chars     int       `json:"chars"`
	CreatedAt time.Time `json:"created_at"`
}

type HistoryStore struct {
	path   string
	mu     sync.Mutex
	items  []HistoryEntry
	nextID int
}

func NewHistoryStore(stateDir string) *HistoryStore {
	h := &HistoryStore{
		path:   filepath.Join(stateDir, "history.json"),
		nextID: 1,
	}
	h.load()
	return h
}

func (h *HistoryStore) Add(entry HistoryEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry.ID = h.nextID
	h.nextID++
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.Chars = len([]rune(entry.Text))
	h.items = append(h.items, entry)
	h.save()
}

func (h *HistoryStore) List(offset, limit int) ([]HistoryEntry, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	total := len(h.items)
	if offset >= total {
		return nil, total
	}
	// Return newest first
	sorted := make([]HistoryEntry, len(h.items))
	copy(sorted, h.items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	end := offset + limit
	if end > total {
		end = total
	}
	return sorted[offset:end], total
}

func (h *HistoryStore) Stats() (sessions int, chars int, duration float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.items {
		chars += e.Chars
		duration += e.Duration
	}
	return len(h.items), chars, duration
}

// DeleteByIDs removes entries by their IDs.
func (h *HistoryStore) DeleteByIDs(ids []int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	idSet := make(map[int]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	filtered := h.items[:0]
	deleted := 0
	for _, item := range h.items {
		if idSet[item.ID] {
			deleted++
			continue
		}
		filtered = append(filtered, item)
	}
	h.items = filtered
	h.save()
	return deleted
}

// DeleteByDateRange removes entries between from and to (inclusive).
func (h *HistoryStore) DeleteByDateRange(from, to time.Time) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	filtered := h.items[:0]
	deleted := 0
	for _, item := range h.items {
		if !item.CreatedAt.Before(from) && !item.CreatedAt.After(to) {
			deleted++
			continue
		}
		filtered = append(filtered, item)
	}
	h.items = filtered
	h.save()
	return deleted
}

// ExportAll returns all entries (newest first) for export.
func (h *HistoryStore) ExportAll() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	sorted := make([]HistoryEntry, len(h.items))
	copy(sorted, h.items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted
}

// DateStats returns distinct dates that have records, grouped by year→month→days.
func (h *HistoryStore) DateStats() map[string]map[string][]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	stats := make(map[string]map[string][]int)
	for _, e := range h.items {
		y := e.CreatedAt.Format("2006")
		m := e.CreatedAt.Format("01")
		d := e.CreatedAt.Format("02")
		if stats[y] == nil {
			stats[y] = make(map[string][]int)
		}
		day, _ := strconv.Atoi(d)
		exists := false
		for _, dd := range stats[y][m] {
			if dd == day {
				exists = true
				break
			}
		}
		if !exists {
			stats[y][m] = append(stats[y][m], day)
		}
	}
	// Sort days within each month
	for y := range stats {
		for m := range stats[y] {
			sort.Ints(stats[y][m])
		}
	}
	return stats
}

// TrimIfExceeds removes oldest entries if count exceeds max.
func (h *HistoryStore) TrimIfExceeds(max int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.items) <= max {
		return 0
	}
	excess := len(h.items) - max
	h.items = h.items[excess:]
	h.save()
	return excess
}

func (h *HistoryStore) load() {
	data, err := os.ReadFile(h.path)
	if err != nil {
		return
	}
	var items []HistoryEntry
	if err := json.Unmarshal(data, &items); err != nil {
		return
	}
	h.items = items
	maxID := 0
	for _, item := range items {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	h.nextID = maxID + 1
}

func (h *HistoryStore) save() {
	data, err := json.MarshalIndent(h.items, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(h.path), 0755)
	os.WriteFile(h.path, data, 0644)
}
