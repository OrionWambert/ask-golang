package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Entry is one saved answer. Prompt holds the user's message only: file and
// piped content is not stored.
type Entry struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Prompt    string    `json:"prompt"`
	Answer    string    `json:"answer"`
	SessionID string    `json:"session_id,omitempty"`
	Model     string    `json:"model,omitempty"`
	CostUSD   float64   `json:"cost_usd,omitempty"`
}

// Store keeps one JSON file per answer in Dir.
type Store struct {
	Dir string
}

// Save writes e and fills in its ID and time.
func (s Store) Save(e *Entry) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	e.Time = time.Now()
	e.ID = e.Time.Format("20060102-150405") + fmt.Sprintf("-%03d", e.Time.Nanosecond()/1e6)
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, e.ID+".json"), data, 0o600)
}

// List returns up to limit entries, newest first. A limit of 0 returns them all.
func (s Store) List(limit int) ([]Entry, error) {
	names, err := s.names()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		entry, err := s.Get(strings.TrimSuffix(name, ".json"))
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	return entries, nil
}

// Get returns the entry with the given ID.
func (s Store) Get(id string) (*Entry, error) {
	if id == "" || filepath.Base(id) != id {
		return nil, fmt.Errorf("invalid history id %q", id)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no history entry %q", id)
	}
	if err != nil {
		return nil, err
	}
	entry := &Entry{}
	if err := json.Unmarshal(data, entry); err != nil {
		return nil, fmt.Errorf("invalid history entry %q: %w", id, err)
	}
	return entry, nil
}

// Clear deletes every entry and returns how many were removed.
func (s Store) Clear() (int, error) {
	names, err := s.names()
	if err != nil {
		return 0, err
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(s.Dir, name)); err != nil {
			return 0, err
		}
	}
	return len(names), nil
}

// names returns the entry file names, newest first. IDs sort by time.
func (s Store) names() ([]string, error) {
	files, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			names = append(names, file.Name())
		}
	}
	slices.Sort(names)
	slices.Reverse(names)
	return names, nil
}
