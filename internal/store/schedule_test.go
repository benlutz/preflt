package store

import (
	"testing"
	"time"
)

func TestSaveAndLoadSchedules_roundtrip(t *testing.T) {
	useTemp(t)

	entry := ScheduleEntry{
		Name:      "morning-routine",
		Mode:      "recurring",
		Frequency: "daily",
		Period:    "morning",
		CreatedAt: time.Now(),
	}
	if err := SaveSchedule(entry); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	entries, err := LoadSchedules()
	if err != nil {
		t.Fatalf("LoadSchedules: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "morning-routine" {
		t.Errorf("expected name 'morning-routine', got %q", entries[0].Name)
	}
	if entries[0].Mode != "recurring" {
		t.Errorf("expected mode 'recurring', got %q", entries[0].Mode)
	}
}

func TestLoadSchedules_emptyWhenMissing(t *testing.T) {
	useTemp(t)

	entries, err := LoadSchedules()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestSaveSchedule_upsertByName(t *testing.T) {
	useTemp(t)

	first := ScheduleEntry{Name: "plants", Mode: "recurring", Frequency: "weekly", On: "sunday", CreatedAt: time.Now()}
	if err := SaveSchedule(first); err != nil {
		t.Fatal(err)
	}

	// Update: change cooldown.
	updated := ScheduleEntry{Name: "plants", Mode: "recurring", Frequency: "weekly", On: "sunday", Cooldown: "7d", CreatedAt: first.CreatedAt}
	if err := SaveSchedule(updated); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadSchedules()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(entries))
	}
	if entries[0].Cooldown != "7d" {
		t.Errorf("expected cooldown '7d' after update, got %q", entries[0].Cooldown)
	}
}

func TestSaveSchedule_multipleEntries(t *testing.T) {
	useTemp(t)

	for _, name := range []string{"a", "b", "c"} {
		if err := SaveSchedule(ScheduleEntry{Name: name, Mode: "pending", CreatedAt: time.Now()}); err != nil {
			t.Fatalf("SaveSchedule %q: %v", name, err)
		}
	}

	entries, err := LoadSchedules()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestRemoveSchedule(t *testing.T) {
	useTemp(t)

	for _, name := range []string{"a", "b", "c"} {
		_ = SaveSchedule(ScheduleEntry{Name: name, Mode: "pending", CreatedAt: time.Now()})
	}

	if err := RemoveSchedule("b"); err != nil {
		t.Fatalf("RemoveSchedule: %v", err)
	}

	entries, err := LoadSchedules()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after remove, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Name == "b" {
			t.Error("entry 'b' should have been removed")
		}
	}
}

func TestRemoveSchedule_noopWhenMissing(t *testing.T) {
	useTemp(t)

	_ = SaveSchedule(ScheduleEntry{Name: "a", Mode: "pending", CreatedAt: time.Now()})
	if err := RemoveSchedule("nonexistent"); err != nil {
		t.Fatalf("RemoveSchedule of missing name should not error: %v", err)
	}

	entries, _ := LoadSchedules()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry to remain, got %d", len(entries))
	}
}
