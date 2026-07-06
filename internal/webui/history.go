package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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
