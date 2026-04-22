package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ScheduleEntry is a user-managed schedule for a checklist, stored in
// ~/.preflt/schedules.json. It is the authoritative source for scheduling;
// a YAML schedule block serves as a fallback for self-contained checklists.
type ScheduleEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path,omitempty"`      // optional path if not in ~/.preflt/
	Mode      string    `json:"mode"`                // pending | date | recurring
	From      string    `json:"from,omitempty"`      // YYYY-MM-DD start date
	Frequency string    `json:"frequency,omitempty"` // daily | weekly | monthly
	On        string    `json:"on,omitempty"`        // weekday name for weekly
	Period    string    `json:"period,omitempty"`    // morning | afternoon | evening (display hint)
	Cooldown  string    `json:"cooldown,omitempty"`  // e.g. "7d", "12h"
	CreatedAt time.Time `json:"created_at"`
}

func schedulesPath() (string, error) {
	home, err := prefltHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "schedules.json"), nil
}

// LoadSchedules returns all schedule entries. Returns nil, nil when the file
// does not exist yet.
func LoadSchedules() ([]ScheduleEntry, error) {
	path, err := schedulesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading schedules: %w", err)
	}
	var entries []ScheduleEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing schedules: %w", err)
	}
	return entries, nil
}

// SaveSchedule upserts a schedule entry by name.
func SaveSchedule(entry ScheduleEntry) error {
	entries, err := LoadSchedules()
	if err != nil {
		return err
	}
	found := false
	for i, e := range entries {
		if e.Name == entry.Name {
			entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}
	return writeSchedules(entries)
}

// RemoveSchedule removes the entry with the given name. No-op if not found.
func RemoveSchedule(name string) error {
	entries, err := LoadSchedules()
	if err != nil {
		return err
	}
	filtered := make([]ScheduleEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name != name {
			filtered = append(filtered, e)
		}
	}
	return writeSchedules(filtered)
}

func writeSchedules(entries []ScheduleEntry) error {
	path, err := schedulesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating preflt home: %w", err)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling schedules: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
