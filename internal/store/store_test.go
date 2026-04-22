package store

import (
	"testing"
	"time"
)

// useTemp points the store at a throwaway directory for the duration of the test.
func useTemp(t *testing.T) {
	t.Helper()
	t.Setenv("PREFLT_HOME", t.TempDir())
}

// ── NewRunState ───────────────────────────────────────────────────────────────

func TestNewRunState(t *testing.T) {
	state := NewRunState("my-list", "/path/to/my-list.yaml", []string{"a", "b", "c"})

	if state.RunID == "" {
		t.Error("expected non-empty run ID")
	}
	if state.ChecklistName != "my-list" {
		t.Errorf("expected name 'my-list', got %q", state.ChecklistName)
	}
	if state.ChecklistPath != "/path/to/my-list.yaml" {
		t.Errorf("expected correct path, got %q", state.ChecklistPath)
	}
	if len(state.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(state.Items))
	}
	for _, item := range state.Items {
		if item.Status != StatusPending {
			t.Errorf("expected status pending, got %q", item.Status)
		}
	}
}

// ── SaveState / LoadLatestState ───────────────────────────────────────────────

func TestSaveAndLoad_roundtrip(t *testing.T) {
	useTemp(t)

	state := NewRunState("cl", "./cl.yaml", []string{"x", "y"})
	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadLatestState("cl")
	if err != nil {
		t.Fatalf("LoadLatestState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected state, got nil")
	}
	if loaded.RunID != state.RunID {
		t.Errorf("run ID mismatch: got %q, want %q", loaded.RunID, state.RunID)
	}
	if loaded.ChecklistName != "cl" {
		t.Errorf("name mismatch: got %q", loaded.ChecklistName)
	}
	if len(loaded.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(loaded.Items))
	}
}

func TestLoadLatestState_noneExist(t *testing.T) {
	useTemp(t)

	state, err := LoadLatestState("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil, got %+v", state)
	}
}

func TestLoadLatestState_returnsNewest(t *testing.T) {
	useTemp(t)

	older := NewRunState("cl", "./cl.yaml", []string{"a"})
	if err := SaveState(older); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond) // ensure a later timestamp

	newer := NewRunState("cl", "./cl.yaml", []string{"a"})
	if err := SaveState(newer); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadLatestState("cl")
	if err != nil {
		t.Fatalf("LoadLatestState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected state, got nil")
	}
	if loaded.RunID != newer.RunID {
		t.Errorf("expected newest run %q, got %q", newer.RunID, loaded.RunID)
	}
}

func TestLoadLatestState_ignoresOtherChecklists(t *testing.T) {
	useTemp(t)

	a := NewRunState("checklist-a", "./a.yaml", []string{"1"})
	b := NewRunState("checklist-b", "./b.yaml", []string{"1"})
	if err := SaveState(a); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(b); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadLatestState("checklist-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.RunID != a.RunID {
		t.Errorf("expected run for checklist-a, got %+v", loaded)
	}
}

// ── CompleteRun ───────────────────────────────────────────────────────────────

func TestCompleteRun_writesLogAndRemovesState(t *testing.T) {
	useTemp(t)

	state := NewRunState("cl", "./cl.yaml", []string{"a", "b"})
	if err := SaveState(state); err != nil {
		t.Fatal(err)
	}
	if err := CompleteRun(state, "testuser", "", ""); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}

	// In-progress state should be gone.
	remaining, _ := LoadLatestState("cl")
	if remaining != nil {
		t.Error("expected state to be removed after completion")
	}

	// Run log should be in history.
	logs, err := LoadHistory("cl", 10)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].CompletedBy != "testuser" {
		t.Errorf("expected completedBy 'testuser', got %q", logs[0].CompletedBy)
	}
	if logs[0].Status != "completed" {
		t.Errorf("expected status 'completed', got %q", logs[0].Status)
	}
	if logs[0].RunID != state.RunID {
		t.Errorf("run ID mismatch in log")
	}
}

// ── LoadHistory ───────────────────────────────────────────────────────────────

func TestLoadHistory_empty(t *testing.T) {
	useTemp(t)

	logs, err := LoadHistory("nobody", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

func TestLoadHistory_sortedNewestFirst(t *testing.T) {
	useTemp(t)

	for i := 0; i < 3; i++ {
		state := NewRunState("cl", "./cl.yaml", []string{"a"})
		if err := CompleteRun(state, "user", "", ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	logs, err := LoadHistory("cl", 10)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
	for i := 1; i < len(logs); i++ {
		if logs[i].CompletedAt.After(logs[i-1].CompletedAt) {
			t.Errorf("logs not sorted newest-first at index %d", i)
		}
	}
}

func TestLoadHistory_respectsLimit(t *testing.T) {
	useTemp(t)

	for i := 0; i < 5; i++ {
		state := NewRunState("cl", "./cl.yaml", []string{"a"})
		if err := CompleteRun(state, "user", "", ""); err != nil {
			t.Fatal(err)
		}
	}

	logs, err := LoadHistory("cl", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 (limit), got %d", len(logs))
	}
}

func TestLoadHistory_ignoresOtherChecklists(t *testing.T) {
	useTemp(t)

	stateA := NewRunState("cl-a", "./a.yaml", []string{"x"})
	stateB := NewRunState("cl-b", "./b.yaml", []string{"x"})
	_ = CompleteRun(stateA, "user", "", "")
	_ = CompleteRun(stateB, "user", "", "")

	logs, err := LoadHistory("cl-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log for cl-a, got %d", len(logs))
	}
}

// ── DiscardState ──────────────────────────────────────────────────────────────

func TestDiscardState(t *testing.T) {
	useTemp(t)

	state := NewRunState("cl", "./cl.yaml", []string{"a"})
	if err := SaveState(state); err != nil {
		t.Fatal(err)
	}
	if err := DiscardState(state); err != nil {
		t.Fatalf("DiscardState: %v", err)
	}

	loaded, _ := LoadLatestState("cl")
	if loaded != nil {
		t.Error("expected state to be discarded, but found one")
	}
}
